<script setup lang="ts">
import { ref } from 'vue'
import { showFailToast, showSuccessToast } from 'vant'
import * as api from '../composables/useApi'

defineProps<{
  visible: boolean
  message: string
}>()

const emit = defineEmits<{
  close: []
  ready: []
}>()

const checking = ref(false)

function handleDownload() {
  api.openNpcapDownloadPage()
}

async function handleRecheck() {
  checking.value = true
  try {
    const status = await api.recheckNpcap()
    if (status.installed) {
      showSuccessToast('Npcap 已就绪')
      emit('ready')
      return
    }
    showFailToast(status.message || '仍未检测到 Npcap')
  } catch (e) {
    console.error('重新检测 Npcap 失败:', e)
    showFailToast('检测失败，请稍后重试')
  } finally {
    checking.value = false
  }
}
</script>

<template>
  <van-popup
    :show="visible"
    position="center"
    round
    :close-on-click-overlay="false"
    :style="{ background: 'rgba(40, 40, 40, 0.98)', minWidth: '320px', maxWidth: '360px' }"
  >
    <div class="npcap-dialog">
      <div class="npcap-dialog-title">需要安装 Npcap</div>
      <div class="npcap-dialog-content">
        <p>{{ message || '未检测到 Npcap，抓包功能需要先安装。' }}</p>
        <p class="npcap-dialog-tip">
          安装时建议勾选 “Install Npcap in WinPcap API-compatible Mode”。
        </p>
      </div>
      <div class="npcap-dialog-buttons">
        <van-button type="primary" size="normal" block @click="handleDownload">
          前往下载
        </van-button>
        <van-button
          plain
          size="normal"
          block
          class="dialog-btn-recheck"
          :loading="checking"
          @click="handleRecheck"
        >
          我已安装，重新检测
        </van-button>
      </div>
    </div>
  </van-popup>
</template>

<style lang="scss" scoped>
.npcap-dialog {
  padding: 20px;
}

.npcap-dialog-title {
  font-size: 16px;
  font-weight: 600;
  color: #fff;
  margin-bottom: 12px;
  text-align: center;
}

.npcap-dialog-content {
  font-size: 13px;
  line-height: 1.6;
  color: #ccc;
  margin-bottom: 16px;

  p {
    margin: 0 0 8px;
  }
}

.npcap-dialog-tip {
  color: #999;
  font-size: 12px;
}

.npcap-dialog-buttons {
  display: flex;
  flex-direction: column;
  gap: 10px;

  .dialog-btn-recheck {
    color: #ccc;
    border-color: rgba(255, 255, 255, 0.2);
  }
}
</style>
