package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/borg-backup-manager/backend/internal/crypto"
	"github.com/borg-backup-manager/backend/internal/db"
)

var envVarNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func strPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func resolveRepoBaseURL(dest *db.Destination) string {
	repoURL := strings.TrimSpace(strPtr(dest.RepoURL))
	if dest.Type == "s3" {
		if strings.HasPrefix(repoURL, "s3://") {
			return strings.TrimRight(repoURL, "/")
		}
		bucket := strings.Trim(strings.TrimSpace(strPtr(dest.S3Bucket)), "/")
		if bucket == "" {
			return ""
		}
		path := strings.Trim(repoURL, "/")
		if path != "" {
			return "s3://" + bucket + "/" + path
		}
		return "s3://" + bucket
	}
	return repoURL
}

func repoPathForJob(dest *db.Destination, job *db.BackupJob) string {
	base := resolveRepoBaseURL(dest)
	if base == "" {
		return ""
	}
	if job == nil {
		return base
	}
	safe := sanitizeJobName(job.Name)
	if strings.HasPrefix(base, "ssh://") || strings.HasPrefix(base, "s3://") {
		if strings.HasSuffix(base, "/") {
			return base + safe
		}
		return base + "/" + safe
	}
	return filepath.Join(base, safe)
}

func isRemoteRepoPath(p string) bool {
	return strings.HasPrefix(p, "s3://") || strings.HasPrefix(p, "ssh://") || strings.HasPrefix(p, "rclone://")
}

func borgRunDir(repoPath string) string {
	if isRemoteRepoPath(repoPath) {
		return "/"
	}
	return ""
}

func (e *Engine) buildBorgEnv(dest *db.Destination) []string {
	env := os.Environ()
	env = append(env,
		"BORG_UNKNOWN_UNENCRYPTED_REPO_ACCESS_IS_OK=yes",
		"BORG_RELOCATED_REPO_ACCESS_IS_OK=yes",
	)
	if dest.PassphraseEnv != nil && *dest.PassphraseEnv != "" {
		if v := os.Getenv(*dest.PassphraseEnv); v != "" {
			env = append(env, "BORG_PASSPHRASE="+v)
		}
	}
	sshDir := filepath.Join(e.cfg.DataDir, "ssh")
	_ = os.MkdirAll(sshDir, 0o700)
	knownHosts := filepath.Join(sshDir, "known_hosts")
	if dest.SSHKeyPath != nil && *dest.SSHKeyPath != "" {
		if keyFile, err := e.resolveSSHKeyFile(context.Background(), *dest.SSHKeyPath); err == nil && keyFile != "" {
			strict := "no"
			if dest.SSHHostKeyChecking {
				strict = "accept-new"
			}
			env = append(env, fmt.Sprintf("BORG_RSH=ssh -i %s -o StrictHostKeyChecking=%s -o UserKnownHostsFile=%s", keyFile, strict, knownHosts))
		}
	}
	if dest.Type == "s3" {
		ak, sk := e.resolveS3CredentialsFromDest(dest)
		if ak != "" {
			env = append(env, "AWS_ACCESS_KEY_ID="+ak)
		}
		if sk != "" {
			env = append(env, "AWS_SECRET_ACCESS_KEY="+sk)
		}
		if dest.S3Region != nil && *dest.S3Region != "" {
			env = append(env, "AWS_DEFAULT_REGION="+*dest.S3Region)
		}
		if dest.S3Endpoint != nil && *dest.S3Endpoint != "" {
			endpoint := strings.TrimRight(*dest.S3Endpoint, "/")
			env = append(env, "AWS_ENDPOINT_URL="+endpoint, "BORG_S3_ENDPOINT="+endpoint)
		}
	}
	return env
}

func (e *Engine) resolveS3CredentialsFromSource(source *db.Source) (access, secret string) {
	return e.resolveS3CredentialFields(source.S3AccessKeyEncrypted, source.S3SecretKeyEncrypted, source.S3AccessKeyEnv, source.S3SecretKeyEnv)
}

func (e *Engine) resolveS3CredentialsFromDest(dest *db.Destination) (access, secret string) {
	return e.resolveS3CredentialFields(dest.S3AccessKeyEncrypted, dest.S3SecretKeyEncrypted, dest.S3AccessKeyEnv, dest.S3SecretKeyEnv)
}

func (e *Engine) resolveS3CredentialFields(accessEnc, secretEnc, accessEnv, secretEnv *string) (access, secret string) {
	if accessEnc != nil && secretEnc != nil && *accessEnc != "" && *secretEnc != "" {
		if a, err := e.encryptor.Decrypt(*accessEnc); err == nil && a != "" {
			access = a
		}
		if s, err := e.encryptor.Decrypt(*secretEnc); err == nil && s != "" {
			secret = s
		}
		if access != "" && secret != "" {
			return access, secret
		}
	}
	accessRef := strPtr(accessEnv)
	if accessRef == "" {
		accessRef = "AWS_ACCESS_KEY_ID"
	}
	secretRef := strPtr(secretEnv)
	if secretRef == "" {
		secretRef = "AWS_SECRET_ACCESS_KEY"
	}
	access, _ = resolveEnvCredential(accessRef)
	secret, _ = resolveEnvCredential(secretRef)
	return access, secret
}

func resolveEnvCredential(ref string) (string, string) {
	if ref == "" {
		return "", ""
	}
	if envVarNameRe.MatchString(ref) {
		if v := os.Getenv(ref); v != "" {
			return v, ref
		}
		if len(ref) >= 16 && !strings.Contains(ref, "_") {
			return ref, ""
		}
		return "", ref
	}
	return ref, ""
}

func (e *Engine) resolveSSHKeyFile(ctx context.Context, keyRef string) (string, error) {
	if keyRef == "" {
		return "", fmt.Errorf("no ssh key")
	}
	if strings.HasPrefix(keyRef, "/") {
		if _, err := os.Stat(keyRef); err == nil {
			return keyRef, nil
		}
	}
	key, err := e.store.GetSSHKeyByName(ctx, keyRef)
	if err != nil {
		if strings.HasPrefix(keyRef, "/") {
			return keyRef, nil
		}
		return "", fmt.Errorf("ssh key %q not found", keyRef)
	}
	plain, err := e.encryptor.Decrypt(key.PrivateKey)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(e.cfg.DataDir, "ssh", "keys")
	_ = os.MkdirAll(dir, 0o700)
	path := filepath.Join(dir, fmt.Sprintf("key-%d", key.ID))
	if err := os.WriteFile(path, []byte(plain), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func sshAddressFamily(store *db.Store, ctx context.Context) string {
	raw := strings.ToLower(strings.TrimSpace(store.GetSetting(ctx, "ssh_address_family")))
	if raw == "" {
		raw = strings.ToLower(strings.TrimSpace(os.Getenv("BACKUP_TOOL_SSH_ADDRESS_FAMILY")))
	}
	switch raw {
	case "inet", "ipv4", "4":
		return "inet"
	case "inet6", "ipv6", "6":
		return "inet6"
	default:
		return ""
	}
}

// EncryptS3Field encrypts plaintext credentials for storage.
func EncryptS3Field(enc *crypto.Encryptor, val string) (*string, error) {
	if val == "" {
		return nil, nil
	}
	out, err := enc.Encrypt(val)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
