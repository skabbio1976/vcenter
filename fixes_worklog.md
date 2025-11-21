# Fixes Worklog

## 2025-11-20
- `WaitForTools` now reads the `guest` property once and returns as soon as VMware Tools report `GUEST_TOOLS_RUNNING`, eliminating the previous logic that always timed out.
- Windows customization helpers (`NewWindowsCustomization*`, `CloneFromRequest`) now support static-IP and DNS overrides even when the VM is not joining a domain, ensuring workgroup machines honor the provided network settings.
- Implemented guest operations helpers using `guest.OperationsManager`: `UploadFileToVM`, `DownloadFileFromVM`, `RunScriptOnVM`, and `UploadAndRunScript` now perform actual file transfers and script execution rather than returning placeholder errors.
- Verified `go test ./...` succeeds after the fixes.

