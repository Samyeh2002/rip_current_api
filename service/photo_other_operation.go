package service

import (
	"fmt"
	"log"
	"net/http"
	"rip_current_mod/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 進行圖片的額外操作 (圖片按讚)

type LikeStruct struct {
	MemberGmail string `gorm:"column:user_id"`
	PhotoName   string `gorm:"column:photo_id"`
}

// 新增 (根據用戶資訊、圖片名稱來按讚)
func Photo_other_operation_post(c *gin.Context) {

	// 變數
	jsonStruct := LikeStruct{}
	var count int64

	// 接收參數
	if err := c.ShouldBindJSON(&jsonStruct); err != nil {
		log.Printf("綁定失敗 原因:%S", err)
		c.JSON(http.StatusBadGateway, gin.H{"message": "綁定失敗"})
		return
	}

	// 確保登入才可按讚
	if jsonStruct.MemberGmail != "" {

		if err := database.MyConnect.Table("rip_current_member").Where("memberGmail = ?", jsonStruct.MemberGmail).Count(&count).Error; err != nil {

			// 帳號不存在
			log.Printf("按讚失敗 原因:%s", err)
			c.JSON(http.StatusBadGateway, gin.H{"message": fmt.Sprintf("按讚失敗 原因:%s", err.Error())})
			return
		}

		if count == 0 {

			// 帳號不存在
			log.Printf("按讚失敗 原因:%s", "帳號不存在")
			c.JSON(http.StatusBadGateway, gin.H{"message": fmt.Sprintf("按讚失敗 原因:%s", "帳號不存在")})
			return
		}

	} else {

		// 未登入
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("按讚失敗 原因:%s", "未登入不能按讚")})
	}

	log.Print(jsonStruct)

	// 新增
	if err := database.MyConnect.Table("rip_current_photo_other").Create(&jsonStruct).Error; err != nil {
		log.Printf("新增到資料庫失敗 原因:%s", err)
		c.JSON(http.StatusBadRequest, gin.H{"message": err}) // http碼需要再修改
		return
	}

	go func() {

		// 欄位修改
		if err := database.MyConnect.Table("rip_current_information").Where("photoName = ?", jsonStruct.PhotoName).Update("likeQuantity", gorm.Expr("likeQuantity + ?", 1)).Error; err != nil {
			log.Printf("資料表:%s 更新失敗失敗 原因:%s", "rip_current_information", err)
		}
	}()

	// 成功
	c.JSON(http.StatusOK, gin.H{"message": "按讚成功"})
}

// 查詢 (指定圖片的按讚數)
func Photo_other_operation_query(c *gin.Context) {

	// 變數
	jsonStruct := LikeStruct{}
	var count int64

	// 接收參數
	if err := c.ShouldBindJSON(&jsonStruct); err != nil {
		log.Printf("綁定失敗 原因:%S", err)
		c.JSON(http.StatusBadGateway, gin.H{"message": "綁定失敗"})
		return
	}

	// 查詢
	if err := database.MyConnect.Table("rip_current_photo_other").Where("photo_id = ?", jsonStruct.PhotoName).Count(&count).Error; err != nil {
		log.Printf("查詢圖片按讚數量失敗 原因:%s", err)
		c.JSON(http.StatusBadRequest, gin.H{"message": err}) // http碼需要再修改
		return
	}

	// 成功
	c.JSON(http.StatusOK, gin.H{"message": count})
}

// 刪除 刪除圖片的全部按讚
func Photo_other_operation_delete(c *gin.Context) {

	// 變數
	jsonStruct := LikeStruct{}

	// 接收參數
	if err := c.ShouldBindJSON(&jsonStruct); err != nil {
		log.Printf("綁定失敗 原因:%S", err)
		c.JSON(http.StatusBadGateway, gin.H{"message": "綁定失敗"})
		return
	}

	// 刪除
	if err := database.MyConnect.Table("rip_current_photo_other").Where("photo_id = ?", jsonStruct.PhotoName).Delete(&LikeStruct{}).Error; err != nil {
		log.Printf("刪除圖片按讚數量失敗 原因:%s", err)
		c.JSON(http.StatusBadRequest, gin.H{"message": err}) // http碼需要再修改
		return
	}

	go func() {

		// 欄位修改 更新為0
		if err := database.MyConnect.Table("rip_current_information").Where("photoName = ?", jsonStruct.PhotoName).Update("likeQuantity", 0).Error; err != nil {
			log.Printf("資料表:%s 更新失敗失敗 原因:%s", "rip_current_information", err)
		}
	}()

	// 成功
	c.JSON(http.StatusOK, gin.H{"message": "刪除成功"})
}

// 修改 (用於取消按讚)
func Photo_other_operation_modify(c *gin.Context) {

	// 變數
	jsonStruct := LikeStruct{}
	var count int64

	// 接收參數
	if err := c.ShouldBindJSON(&jsonStruct); err != nil {
		log.Printf("綁定失敗 原因:%S", err)
		c.JSON(http.StatusBadGateway, gin.H{"message": "綁定失敗"})
		return
	}

	log.Printf("%s 想對 %s 進行刪除的動作", jsonStruct.MemberGmail, jsonStruct.PhotoName)

	// 確保登入才可倒讚
	if jsonStruct.MemberGmail != "" {

		if err := database.MyConnect.Table("rip_current_photo_other").Where("user_id = ? AND photo_id = ?", jsonStruct.MemberGmail, jsonStruct.PhotoName).Count(&count).Error; err != nil {

			// 帳號不存在
			log.Printf("倒讚失敗 原因:%s", err)
			c.JSON(http.StatusBadGateway, gin.H{"message": fmt.Sprintf("倒讚失敗 原因:%s", err.Error())})
			return
		}

		if count == 0 {

			// 帳號不存在
			log.Printf("倒讚失敗 原因:%s", "目標不存在")
			c.JSON(http.StatusBadGateway, gin.H{"message": fmt.Sprintf("倒讚失敗 原因:%s", "目標不存在")})
			return
		}

	} else {

		// 未登入
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("倒讚失敗 原因:%s", "未登入不能倒讚")})
	}

	// 刪除
	if err := database.MyConnect.Table("rip_current_photo_other").Where("user_id = ? AND photo_id = ?", jsonStruct.MemberGmail, jsonStruct.PhotoName).Delete(&LikeStruct{}).Error; err != nil {
		log.Printf("刪除圖片按讚數量失敗 原因:%s", err)
		c.JSON(http.StatusBadRequest, gin.H{"message": err}) // http碼需要再修改
		return
	}

	go func() {

		// 欄位修改
		if err := database.MyConnect.Table("rip_current_information").Where("photoName = ?", jsonStruct.PhotoName).Update("likeQuantity", gorm.Expr("likeQuantity - ?", 1)).Error; err != nil {
			log.Printf("資料表:%s 更新失敗失敗 原因:%s", "rip_current_information", err)
		}
	}()

	// 成功
	c.JSON(http.StatusOK, gin.H{"message": "刪除成功"})
}
