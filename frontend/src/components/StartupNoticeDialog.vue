<script setup lang="ts">
import { computed } from 'vue'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
import { sanitizeAnnouncementHtml } from '../utils/sanitizeAnnouncementHtml'

const props = defineProps<{
  visible: boolean
  kind: 'announcement' | 'ranking'
  announcement?: ServerAnnouncement | null
}>()

const emit = defineEmits<{
  (event: 'confirm'): void
}>()

const title = computed(() => {
  if (props.kind === 'ranking') return '公开排行说明'
  return props.announcement?.title?.trim() || '服务端公告'
})

const safeAnnouncementHtml = computed(() => (
  sanitizeAnnouncementHtml(props.announcement?.html || '')
))

function handleAnnouncementClick(event: MouseEvent) {
  const target = event.target as HTMLElement
  const link = target.closest<HTMLAnchorElement>('a[href]')
  if (!link) return
  event.preventDefault()
  if (window.runtime) {
    BrowserOpenURL(link.href)
  } else {
    window.open(link.href, '_blank', 'noopener,noreferrer')
  }
}
</script>

<template>
  <van-popup
    :show="visible"
    position="center"
    round
    :close-on-click-overlay="false"
    :close-on-popstate="false"
    class="startup-notice-popup"
  >
    <section class="startup-notice" role="dialog" aria-modal="true" :aria-label="title">
      <header class="startup-notice-header">
        <van-icon :name="kind === 'announcement' ? 'volume-o' : 'bar-chart-o'" aria-hidden="true" />
        <h2>{{ title }}</h2>
      </header>

      <div
        v-if="kind === 'announcement'"
        class="startup-notice-html"
        @click="handleAnnouncementClick"
        v-html="safeAnnouncementHtml"
      />
      <div v-else class="ranking-intro">
        <p>部分玩家希望战斗表现保持低调，因此排行参与方式改为由每位玩家自主选择。</p>
        <p>你可以选择不参与、匿名排行或公开排行。匿名排行不会展示角色名，公开排行会展示角色名。</p>
        <p>百百机器人的排行查询功能将于近日开放，现在可以先完成偏好设置。</p>
      </div>

      <footer class="startup-notice-actions">
        <van-button type="primary" block @click="emit('confirm')">
          {{ kind === 'ranking' ? '确定并前往设置' : '确定' }}
        </van-button>
      </footer>
    </section>
  </van-popup>
</template>

<style lang="scss" scoped>
.startup-notice-popup {
  width: min(360px, calc(100vw - 32px));
  max-height: min(480px, calc(100vh - 48px));
  background: rgba(36, 36, 36, 0.99);
}

.startup-notice {
  display: flex;
  max-height: min(480px, calc(100vh - 48px));
  flex-direction: column;
  padding: 18px;
}

.startup-notice-header {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 8px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  color: #64b5f6;

  h2 {
    min-width: 0;
    margin: 0;
    color: #fff;
    font-size: 16px;
    font-weight: 600;
    line-height: 1.4;
    overflow-wrap: anywhere;
  }
}

.startup-notice-html,
.ranking-intro {
  min-height: 0;
  margin: 0;
  padding: 14px 2px;
  overflow-y: auto;
  color: #d0d0d0;
  font-size: 13px;
  line-height: 1.7;
}

.ranking-intro p {
  margin: 0 0 10px;

  &:last-child {
    margin-bottom: 0;
  }
}

.startup-notice-actions {
  flex: 0 0 auto;
  padding-top: 4px;
}

:deep(.startup-notice-html) {
  h1,
  h2,
  h3,
  h4 {
    margin: 10px 0 6px;
    color: #fff;
    font-size: 14px;
    letter-spacing: 0;
  }

  p,
  ul,
  ol,
  blockquote,
  pre,
  table {
    margin: 0 0 10px;
  }

  a {
    color: #64b5f6;
    text-decoration: underline;
  }

  blockquote {
    padding-left: 10px;
    border-left: 3px solid rgba(100, 181, 246, 0.6);
    color: #aaa;
  }

  pre,
  code {
    border-radius: 4px;
    background: rgba(0, 0, 0, 0.3);
    font-family: Consolas, monospace;
  }

  pre {
    padding: 8px;
    overflow-x: auto;
  }

  table {
    width: 100%;
    border-collapse: collapse;
  }

  th,
  td {
    padding: 5px 6px;
    border: 1px solid rgba(255, 255, 255, 0.15);
    text-align: left;
  }
}
</style>
