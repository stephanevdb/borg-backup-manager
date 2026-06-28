package auth

func (s *Service) OAuthEnabled() bool {
	return s.oauth != nil
}
