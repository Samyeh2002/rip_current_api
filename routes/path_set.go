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

	// 照片操作使用
	r.Group("/").POST("/photo/post", service.Photo_operation_post_one)                                 // 新增圖片
	r.Group("/").GET("/photo/get/one/:filename", service.Photo_operation_get_one)                      // 從資料夾找一張圖片
	r.Group("/").GET("/photo/get/information/:filename", service.Photo_operation_get_information)      // 從資料庫取得圖片資訊
	r.Group("/").GET("/photo/get/folder", service.Photo_operation_get_folder)                          // 從資料庫取回多張圖片
	r.Group("/").POST("/photo/get/folder/information", service.Photo_operation_get_folder_information) // 從資料庫取回多張圖片的資訊

	// 成員操作使用
	r.Group("/").POST("/member/insert", service.Member_Insert)          // 新增
	r.Group("/").POST("member/query/login", service.Member_Query)       // 查詢
	r.Group("/").POST("member/query/exist", service.Member_Query_Exist) // 查詢(帳號是否存在)
	r.Group("/").POST("member/delete", service.Member_Delete)           // 刪除
	r.Group("/").POST("member/modify", service.Member_Modify)           // 修改

	// 照片按讚操作
	r.Group("").POST("/photo/other/like", service.Photo_other_operation_post)     // 新增
	r.Group("").POST("/photo/other/query", service.Photo_other_operation_query)   // 查詢
	r.Group("").POST("/photo/other/delete", service.Photo_other_operation_delete) // 刪除
	r.Group("").POST("/photo/other/delike", service.Photo_other_operation_modify) // 修改
}
