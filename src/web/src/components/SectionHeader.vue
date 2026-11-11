<script setup>
defineProps({
  title: { type: String, required: true },
  subtitle: { type: String, default: '' },
  to: { type: [String, Object], default: null },
  moreText: { type: String, default: '查看全部' },
})
</script>

<template>
  <div class="section-header">
    <div class="section-title">
      <h2>{{ title }}</h2>
      <span v-if="subtitle" class="section-subtitle">{{ subtitle }}</span>
    </div>
    <slot name="actions"></slot>
    <router-link v-if="to" :to="to" class="section-more">
      {{ moreText }} <i class="ri-arrow-right-s-line"></i>
    </router-link>
  </div>
</template>
