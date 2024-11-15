package templates

import "sync"

// 常用
var GlobalRootPath string = "C:\\Users\\PC3\\Documents\\Rip_Current_Photo" // 圖片資料夾的根目錄
var GlobalPythonPhotoPath string = "C:\\Users\\PC3\\Pictures\\test"        // Python 腳本位置

// 網頁資料夾路徑
var WebLoadPath string = "C:\\Users\\PC3\\Documents\\web_photo\\Load\\load.zip"             // 網頁載入時用
var WebCompletePath string = "C:\\Users\\PC3\\Documents\\web_photo\\Complete\\complete.zip" // 網頁完整用
var WebLoadPathQuantity int = 10                                                            // 設定網頁載入時的數量

// 緩存
var OperationCache sync.Map      // 管理員專用
var UserOTPCache sync.Map        // 一般用戶使用
var PasswordForgotCache sync.Map // 忘記密碼用
