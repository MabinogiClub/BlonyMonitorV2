/**
 * Window 对象扩展
 * 添加 Wails 运行时和 Go 后端 API
 */

import * as AppBindings from '../../wailsjs/go/app/App'

export {}

declare global {
  interface Window {
    runtime: WailsRuntime
    go: {
      app: {
        App: typeof AppBindings
      }
    }
  }
}
