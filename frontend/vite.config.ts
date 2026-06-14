import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { VantResolver } from '@vant/auto-import-resolver'

/**
 * Vite 配置文件
 * 用于构建 Vue 3 前端应用
 */
export default defineConfig({
  plugins: [
    vue() as any,
    AutoImport({
      dts: 'src/types/auto-imports.d.ts',
      resolvers: [VantResolver()],
    }),
    Components({
      dts: 'src/types/components.d.ts',
      resolvers: [VantResolver()],
    }),
  ],
  
  // 解析配置
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src')
    }
  },
  
  // 构建配置
  build: {
    // 输出到 dist 目录
    outDir: 'dist',
    // 清空输出目录
    emptyOutDir: true,
    // 生成 sourcemap 便于调试
    sourcemap: false,
    // 资源内联阈值
    assetsInlineLimit: 4096,
    // Rollup 配置
    rollupOptions: {
      output: {
        // 入口文件名
        entryFileNames: 'js/[name].js',
        // 代码分割后的文件名
        chunkFileNames: 'js/[name].js',
        // 静态资源文件名
        assetFileNames: (assetInfo) => {
          const info = assetInfo.name || ''
          if (info.endsWith('.css')) {
            return 'styles/[name][extname]'
          }
          return 'assets/[name][extname]'
        }
      }
    }
  },
  
  // 开发服务器配置
  server: {
    port: 5173,
    host: true
  },
  
  // 基础路径（Wails 应用使用相对路径）
  base: './'
})
