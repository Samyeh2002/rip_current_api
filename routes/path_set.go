package routes

import (
	"rip_current_mod/service"

	"github.com/gin-gonic/gin"
)

func Path_set(r *gin.RouterGroup) {

	// 內部測試使用
	r.Group("/").GET("/test/get/:url", service.Connect_test_get)
	r.Group("/").GET("/test/get/test", service.Connect_test_get_other)
	r.Group("/").GET("/test/get/check", service.Connect_test_get_check)
	r.Group("/").POST("/test/get/python", service.Call_Python)
	r.Group("/").GET("/test/get/gmail", service.SendGmailTest)
	r.Group("/").GET("/test/get/sync", service.SyncTest)

	// 照片操作使用
	r.Group("/").POST("/photo/post", service.Photo_operation_post_one)                                          // 功能1-新增單張圖片及圖片資訊
	r.Group("/").GET("/photo/get/one/:filename", service.Photo_operation_get_one)                               // 功能2-從資料夾抓一張圖片不包含資訊
	r.Group("/").GET("/photo/get/information/:filename", service.Photo_operation_get_information)               // 功能3-從資料庫取得圖片資訊
	r.Group("/").POST("/photo/get/folder/information", service.Photo_operation_get_folder_information)          // 功能5-從資料庫取回多張圖片的資訊
	r.Group("/").GET("/photo/get/folder/select", service.Photo_operation_complete_folder)                       // 功能6-根據參數
	r.Group("/").GET("/photo/get/folder/select/:filename", service.Photo_operation_get_one_include_information) // 功能5-從資料庫取回多張圖片的資訊
	r.Group("/").POST("/photo/post/painting", service.Photo_operation_painting_rip)                             // 功能: 劃出離岸流
	// r.Group("/").GET("/photo/get/folder", service.Photo_operation_get_folder)                          		// 功能4-從指定的資料夾中取得全部圖片

	// 成員操作使用
	r.Group("/").POST("/member/insert", service.Member_Insert)                     // 新增
	r.Group("/").POST("member/query/login", service.Member_Query)                  // 查詢
	r.Group("/").POST("member/query/exist", service.Member_Query_Exist)            // 查詢(帳號是否存在)
	r.Group("/").POST("member/delete", service.Member_Delete)                      // 刪除{}
	r.Group("/").POST("member/modifyuserinformation", service.Member_Modify_Other) // 修改
	r.Group("/").POST("member/modifyreset", service.Member_Modify_Password)
	r.Group("/").Any("member/passwordforgot/check", service.Member_Modify_PasswordForgot_check)  // 忘記密碼 透過Gmail修改 檢查
	r.Group("/").POST("member/passwordforgot/reset", service.Member_Modify_PasswordForgot_Reset) // 忘記密碼 透過Gmail修改 正式

	// 管理員操作
	r.Group("/").POST("/administrator/insert", service.InsertAdministrator)      // 新增
	r.Group("/").POST("/administrator/delete", service.DeleteAdministrator)      // 刪除
	r.Group("/").POST("/administrator/operation", service.Permissions_Operation) // 權限操作
	r.Group("/").POST("/administrator/otp", service.Permissions_OTP_Check)       // 權限操作

	// 照片按讚操作
	r.Group("").POST("/photo/other/like", service.Photo_other_operation_post)     // 新增
	r.Group("").POST("/photo/other/query", service.Photo_other_operation_query)   // 查詢
	r.Group("").POST("/photo/other/delete", service.Photo_other_operation_delete) // 刪除
	r.Group("").POST("/photo/other/delike", service.Photo_other_operation_modify) // 修改

	// 網頁
	r.Group("/").GET("/web/photo", service.Web_Photo_classification)
}
