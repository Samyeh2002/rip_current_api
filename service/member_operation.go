package service

import (
	"log"
	"net/http"
	"rip_current_mod/database"

	"github.com/gin-gonic/gin"
)

// 結構
type NumberStruct struct {
	MemberGmail    string `gorm:"column:memberGmail"`
	MemberName     string `gorm:"column:memberName"`
	MemberPhone    string `gorm:"column:memberPhone;default:null"`
	MemberPassword string `gorm:"column:memberPassword"`
}

// 新增
func Member_Insert(c *gin.Context) {

	// 變數
	RegInfo := NumberStruct{}

	// 接收參數
	if err := c.ShouldBindJSON(&RegInfo); err != nil {
		log.Printf("綁定失敗 原因:%S", err)
		c.JSON(200, gin.H{"message": "綁定失敗"})
		return
	}

	// 新增到資料庫
	if err := database.MyConnect.Table("rip_current_member").Create(&RegInfo).Error; err != nil {
		log.Print("新增到資料庫失敗")
		c.JSON(200, gin.H{"message": "新增到資料庫失敗"})
		return
	}

	// 成功
	log.Printf("帳號:%s 新增到資料庫成功", RegInfo.MemberGmail)
	c.JSON(200, gin.H{"message": "帳號新增成功"})
}

// 查詢(帳號登入)
func Member_Query(c *gin.Context) {

	log.Print("Hello world")

	// 變數
	SearchInfo := NumberStruct{}
	checkInfo := NumberStruct{}
	var count int64

	// 接收參數
	if err := c.ShouldBindJSON(&SearchInfo); err != nil {
		log.Printf("綁定失敗 原因:%S", err)
		c.JSON(http.StatusBadGateway, gin.H{"message": "綁定失敗"})
		return
	}

	// 尋找目標
	if err := database.MyConnect.Table("rip_current_member").Select("memberPassword").Where("memberGmail = ?", SearchInfo.MemberGmail).Count(&count).Find(&checkInfo).Error; err != nil {
		log.Printf("調用資料庫出現問題:%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "查詢錯誤"})
		return
	}

	// 結果
	if count > 0 {

		// 帳號存在 檢查密碼
		if SearchInfo.MemberPassword == checkInfo.MemberPassword {

			// 成功 目標存在
			log.Printf("帳號:%s 密碼正確", SearchInfo.MemberGmail)
			c.JSON(http.StatusOK, gin.H{"message": "密碼正確"})
		} else {

			// 失敗 目標不存在
			log.Printf("帳號:%s 密碼錯誤", SearchInfo.MemberGmail)
			c.JSON(http.StatusUnauthorized, gin.H{"message": "密碼不正確"})
		}
	} else {

		// 帳號錯誤
		log.Printf("帳號:%s 帳號不存在", SearchInfo.MemberGmail)
		c.JSON(http.StatusNotFound, gin.H{"message": "帳號不存在"})
	}
}

// 查詢(帳號是否存在)
func Member_Query_Exist(c *gin.Context) {

	// 變數
	checkInfo := NumberStruct{}
	var count int64

	// 結合
	if err := c.ShouldBindJSON(&checkInfo); err != nil {
		log.Printf("綁定失敗 原因:%S", err)
		c.JSON(200, gin.H{"message": "綁定失敗"})
		return
	}

	// 查詢
	if err := database.MyConnect.Table("rip_current_member").Where("memberGmail = ?", checkInfo.MemberGmail).Count(&count).Error; err != nil {

		// 發生錯誤
		log.Printf("查詢帳號:%s時發生錯誤 原因:%s", checkInfo.MemberGmail, err)
		c.JSON(400, gin.H{"message": "查詢資料庫時發生錯誤"})
		return
	}

	// 查詢成功
	if count > 0 {

		// 帳號存在
		c.JSON(200, gin.H{"message": "帳號存在"})
		return
	}

	// 帳號不存在
	c.JSON(200, gin.H{"message": "帳號不存在"})
}

// 刪除
func Member_Delete(c *gin.Context) {

	// 變數
	DeleteInfo := NumberStruct{}

	// 接收參數
	if err := c.ShouldBindJSON(&DeleteInfo); err != nil {
		log.Printf("綁定失敗 原因:%S", err)
		c.JSON(200, gin.H{"message": "綁定失敗"})
		return
	}

	// 呼叫DB
	if err := database.MyConnect.Table("rip_current_member").Where("memberGmail = ? AND memberPassword = ?", DeleteInfo.MemberGmail, DeleteInfo.MemberPassword).Delete(&DeleteInfo).Error; err != nil {
		log.Printf("帳號:%s 刪除失敗 原因:%s", DeleteInfo.MemberGmail, err)
		c.JSON(400, gin.H{"message": "帳號刪除失敗"})
		return
	}

	// 結果
	log.Printf("帳號:%s 刪除成功", DeleteInfo.MemberGmail)
	c.JSON(200, gin.H{"message": "帳號刪除成功"})
}

// 修改 (錯誤 不應該允許密碼修改)
func Member_Modify(c *gin.Context) {

	// 變數
	ModifyInfo := NumberStruct{}

	// 接收參數
	if err := c.ShouldBindJSON(&ModifyInfo); err != nil {
		log.Printf("綁定失敗 原因:%S", err)
		c.JSON(400, gin.H{"message": "綁定失敗"})
		return
	}

	// 修改
	if err := database.MyConnect.Table("rip_current_member").Model(&ModifyInfo).Where("memberGmail = ?", ModifyInfo.MemberGmail).Updates(&ModifyInfo).Error; err != nil {
		log.Printf("帳號:%s 修改資訊失敗 原因:%s", ModifyInfo.MemberGmail, err)
		c.JSON(400, gin.H{"message": "修改資訊失敗"})
		return
	}

	// 結果
	log.Printf("帳號:%s 修改資訊成功", ModifyInfo.MemberGmail)
	c.JSON(200, gin.H{"message": "修改資訊成功"})
}
