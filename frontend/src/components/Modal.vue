<script setup lang="ts">
defineProps<{ modelValue: boolean; title: string; wide?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [boolean]; save: [] }>()

function close() {
  emit('update:modelValue', false)
}

function save() {
  emit('save')
  close()
}
</script>

<template>
  <div v-if="modelValue" class="modal">
    <div class="modal-backdrop" @click="close" />
    <div class="modal-content" :class="{ 'modal-wide': wide }">
      <div class="modal-header">
        <h3>{{ title }}</h3>
        <button type="button" class="modal-close" @click="close">&times;</button>
      </div>
      <div class="modal-body"><slot /></div>
      <div class="modal-footer">
        <button type="button" class="btn btn-outline" @click="close">Cancel</button>
        <button type="button" class="btn btn-primary" @click="save">Save</button>
      </div>
    </div>
  </div>
</template>
