# Desktop Build Assets

[`Taskfile.yml`](../Taskfile.yml) defines the Wails 3 build and package tasks. [`config.yml`](config.yml) configures the desktop development watcher.

- Set the default version in [`VERSION`](../VERSION). The release workflows pass `APP_VERSION` to the Go linker and generated package metadata.
- Edit the application artwork in `appicon.png` and the Windows icon in `windows/icon.ico`. Wails generates macOS icons, Info.plists, Windows resources and NSIS sources in `build/.wails/`. Desktop executables and packages go to `build/bin/`.
- The macOS release workflow wraps the `.app` bundle in a DMG and includes the repair scripts from `dmg/`. The Linux release workflows reuse `linux/YT-GO.desktop` for the AppImage and DEB launchers.

# 桌面构建资源

[`Taskfile.yml`](../Taskfile.yml) 定义 Wails 3 的构建和打包任务。[`config.yml`](config.yml) 配置桌面开发模式的文件监听。

- 在 [`VERSION`](../VERSION) 中设置默认应用版本。发布流程通过 `APP_VERSION` 同时设置 Go 程序版本和软件包元数据。
- 在 `appicon.png` 和 `windows/icon.ico` 中维护应用图标。Wails 将生成的 macOS 图标、Info.plist、Windows 资源和 NSIS 脚本写入 `build/.wails/`，桌面程序和安装包位于 `build/bin/`。
- macOS 发布流程将 `.app` 和 `dmg/` 中的修复脚本打包为 DMG。Linux 发布流程使用 `linux/YT-GO.desktop` 生成 AppImage 和 DEB 的启动项。
