<script setup>
import { computed } from 'vue'
import { CheckCircle, XCircle, AlertCircle, Info, X } from 'lucide-vue-next'

const props = defineProps({
  show: {
    type: Boolean,
    default: false
  },
  type: {
    type: String,
    default: 'success', // success, error, warning, info
    validator: (value) => ['success', 'error', 'warning', 'info'].includes(value)
  },
  title: {
    type: String,
    required: true
  },
  message: {
    type: String,
    default: ''
  },
  confirmText: {
    type: String,
    default: 'OK'
  },
  showCancel: {
    type: Boolean,
    default: false
  },
  cancelText: {
    type: String,
    default: 'Cancel'
  },
  isDark: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['close', 'confirm', 'cancel'])

const iconConfig = computed(() => {
  const configs = {
    success: {
      icon: CheckCircle,
      color: 'text-green-500',
      bgColor: 'bg-green-50 dark:bg-green-950/20',
      borderColor: 'border-green-200 dark:border-green-900'
    },
    error: {
      icon: XCircle,
      color: 'text-red-500',
      bgColor: 'bg-red-50 dark:bg-red-950/20',
      borderColor: 'border-red-200 dark:border-red-900'
    },
    warning: {
      icon: AlertCircle,
      color: 'text-amber-500',
      bgColor: 'bg-amber-50 dark:bg-amber-950/20',
      borderColor: 'border-amber-200 dark:border-amber-900'
    },
    info: {
      icon: Info,
      color: 'text-blue-500',
      bgColor: 'bg-blue-50 dark:bg-blue-950/20',
      borderColor: 'border-blue-200 dark:border-blue-900'
    }
  }
  return configs[props.type]
})

const handleClose = () => {
  emit('close')
}

const handleConfirm = () => {
  emit('confirm')
  emit('close')
}

const handleCancel = () => {
  emit('cancel')
  emit('close')
}
</script>

<template>
  <Teleport to="body">
    <!-- Modal Overlay -->
    <Transition name="modal-fade">
      <div
        v-if="show"
        class="jc-modal-overlay fixed inset-0 z-[10000] flex items-center justify-center p-4"
        :class="{ 'is-dark': isDark }"
        @click.self="handleClose"
      >
        <!-- Modal Container -->
        <Transition name="modal-scale">
          <div
            v-if="show"
            :class="['jc-modal', `modal-${type}`, { 'is-dark': isDark }]"
          >
          <!-- Close Button -->
          <button
            @click="handleClose"
            class="jc-modal-close absolute top-4 right-4 p-2 rounded-full transition-colors"
          >
            <X :size="20" />
          </button>

          <!-- Modal Content -->
          <div class="p-6 sm:p-8">
            <!-- Icon -->
            <div :class="['w-16 h-16 rounded-full flex items-center justify-center mx-auto mb-4', iconConfig.bgColor, iconConfig.borderColor, 'border-2']">
              <component :is="iconConfig.icon" :size="32" :class="iconConfig.color" />
            </div>

            <!-- Title -->
            <h3 class="jc-modal-title text-2xl font-bold text-center mb-2">
              {{ title }}
            </h3>

            <!-- Message -->
            <p v-if="message" class="jc-modal-message text-center mb-6">
              {{ message }}
            </p>

            <!-- Slot for custom content -->
            <div v-if="$slots.default" class="mb-6">
              <slot></slot>
            </div>

            <!-- Action Buttons -->
            <div class="flex gap-3">
              <button
                v-if="showCancel"
                @click="handleCancel"
                class="jc-modal-cancel flex-1 px-6 py-3 rounded-xl font-semibold transition-all"
              >
                {{ cancelText }}
              </button>
              <button
                @click="handleConfirm"
                :class="[
                  'px-6 py-3 rounded-xl font-semibold transition-all hover:scale-105',
                  showCancel ? 'flex-1' : 'w-full',
                  type === 'success' ? 'bg-green-500 hover:bg-green-600 text-white' :
                  type === 'error' ? 'bg-red-500 hover:bg-red-600 text-white' :
                  type === 'warning' ? 'bg-amber-500 hover:bg-amber-600 text-white' :
                  'bg-blue-500 hover:bg-blue-600 text-white'
                ]"
              >
                {{ confirmText }}
              </button>
            </div>
          </div>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.jc-modal-overlay {
  background: rgba(20, 24, 18, 0.42);
  backdrop-filter: blur(7px);
}

.jc-modal-overlay.is-dark {
  background: rgba(0, 0, 0, 0.68);
}

.jc-modal {
  position: relative;
  width: 100%;
  max-width: 28rem;
  overflow: hidden;
  border: 1px solid #e6e8e2;
  border-radius: 1rem;
  background: #fff;
  color: #212121;
  box-shadow: 0 24px 70px rgba(25, 32, 20, 0.18);
}

.jc-modal.is-dark {
  border-color: rgba(255, 255, 255, 0.11);
  background: #191a18;
  color: #fafaf7;
  box-shadow: 0 30px 90px rgba(0, 0, 0, 0.58);
}

.jc-modal-close { color: #6f756b; }
.jc-modal-close:hover { background: #f0f1ed; color: #212121; }
.jc-modal.is-dark .jc-modal-close { color: #a8aaa2; }
.jc-modal.is-dark .jc-modal-close:hover { background: #292b27; color: #fff; }
.jc-modal-title { color: #212121; }
.jc-modal-message { color: #666d62; }
.jc-modal.is-dark .jc-modal-title { color: #fafaf7; }
.jc-modal.is-dark .jc-modal-message { color: #b5b7af; }
.jc-modal-cancel { background: #eceee9; color: #212121; }
.jc-modal-cancel:hover { background: #dfe2dc; }
.jc-modal.is-dark .jc-modal-cancel { background: #292b27; color: #fafaf7; }
.jc-modal.is-dark .jc-modal-cancel:hover { background: #343730; }
.jc-modal.is-dark.modal-success :deep(.bg-green-50) { background: rgba(155, 255, 0, 0.09); border-color: rgba(155, 255, 0, 0.28); }
.jc-modal.is-dark.modal-error :deep(.bg-red-50) { background: rgba(255, 82, 82, 0.09); border-color: rgba(255, 112, 112, 0.28); }
.jc-modal.is-dark.modal-warning :deep(.bg-amber-50) { background: rgba(255, 184, 77, 0.09); border-color: rgba(255, 184, 77, 0.28); }
.jc-modal.is-dark.modal-info :deep(.bg-blue-50) { background: rgba(73, 144, 255, 0.09); border-color: rgba(73, 144, 255, 0.28); }

/* Modal Fade Animation */
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.3s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

/* Modal Scale Animation */
.modal-scale-enter-active,
.modal-scale-leave-active {
  transition: all 0.3s ease;
}

.modal-scale-enter-from,
.modal-scale-leave-to {
  opacity: 0;
  transform: scale(0.9);
}
</style>
