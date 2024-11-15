package service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"rip_current_mod/database"
	"rip_current_mod/templates"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/zerobounce/zerobouncego"
	"golang.org/x/crypto/argon2"
)

// 簡介:密碼使用Argon2做加密

// 結構
type NumberStruct struct {
	MemberGmail     string `gorm:"column:memberGmail"`
	MemberName      string `gorm:"column:memberName"`
	MemberPhone     string `gorm:"column:memberPhone;default:nUlL"`
	MemberPassword  string `gorm:"column:memberPassword"`
	MemberOTPSecret string `gorm:"column:userOTPSecret"`
}

type SaltStruct struct {
	UserID       string `gorm:"column:userID"`
	PasswordSalt string `gorm:"column:passwordSalt"`
}

// 用於密碼修改
type ModifyPassword struct {
	UserID       string `gorm:"column:userID"`
	UserPassword string `gorm:"column:memberPassword"`
	NewPassword  string `gorm:"column:memberPassword"`
}

type ZeroBounceResponse struct {
	Address    string `json:"address"`
	Status     string `json:"status"`
	SubStatus  string `json:"sub_status"`
	FreeEmail  bool   `json:"free_email"`
	DidYouMean string `json:"did_you_mean"`
	Account    string `json:"account"`
	Domain     string `json:"domain"`
}

// 系統內部的測試
type UserOTPstruct struct {
	UserID           string
	UserOTPsecret    string
	VerificationCode string
	InsertTime       time.Time
}

// 忘記密碼
type UserNewPasswordstruct struct {
	Passcode    string
	NewPassword string
}

// 用戶登入回應
type AccountLoginReturn struct {
	Message            string // 訊息
	State              string // 0:正常 1:錯誤 2:帳號不存在 3:密碼不正確
	LoginState         string // 0:登入失敗 1:登入成功
	AdministratorState string // 0:不是管理員 1:是管理員
}

// 忘記密碼回應
type PasswordForgetResponse struct {
	Message string // 訊息
	State   string // 0:沒事 1:有問題 2:重複請求
}

// 新增使用者到資料庫，使用Argon2加密 ver.1完成
func Member_Insert(c *gin.Context) {

	// 變數
	RegInfo := NumberStruct{}
	tx := database.MyConnect.Begin() // 紀錄資料庫狀態
	saltStruct := SaltStruct{}

	// 接收參數
	if err := c.ShouldBindJSON(&RegInfo); err != nil {
		c.JSON(400, gin.H{"message": "綁定失敗"})
		log.Printf("綁定失敗 原因:%S", err)
		return
	}

	// 資訊檢查
	if err := Account_Information_Check(RegInfo, 1, c.ClientIP()); err != nil {

		// 發生錯誤
		c.JSON(400, gin.H{"message": fmt.Sprint(err)})
		log.Print(err)
		return
	}

	// 雜湊
	password, salt, err := HashArgon2(RegInfo.MemberPassword)
	if err != nil {

		// 錯誤
		c.JSON(400, gin.H{"錯誤": "新增帳號失敗"})
		log.Printf("錯誤:%v", err)
		return

	} else {

		// 賦值
		saltStruct.UserID = RegInfo.MemberGmail // 外鍵要和主鍵一樣
		saltStruct.PasswordSalt = salt          // 鹽值 之後驗證密碼時還會用到
		RegInfo.MemberPassword = password

		// 產生TOTP
		// 生成一個新的TOTP秘密（類似於用戶註冊時生成）
		key, err := totp.Generate(totp.GenerateOpts{
			Issuer:      "Alderonius",
			AccountName: "Alderonius2002@gmail.com",
		})

		// 發生錯誤
		if err != nil {

			c.JSON(200, gin.H{"message": "新增用戶失敗"})
			log.Print("Error generating key:", err)
			return
		}

		// 添加
		RegInfo.MemberOTPSecret = key.Secret()
	}

	// 新增到資料庫
	if err := database.MyConnect.Table("rip_current_member").Create(&RegInfo).Error; err != nil {
		log.Printf("新增到資料庫失敗 原因:%v", err)
		c.JSON(200, gin.H{"message": "新增到資料庫失敗"})
		tx.Rollback()
		return
	}

	// 新增鹽值到資料庫
	if err := database.MyConnect.Table("rip_current_salt").Create(saltStruct).Error; err != nil {
		log.Print("新增到資料庫失敗")
		c.JSON(200, gin.H{"message": "新帳號的鹽值新增到資料庫失敗"})
		tx.Rollback()
		return
	}

	// 成功
	log.Printf("帳號:%s 新增到資料庫成功", RegInfo.MemberGmail)
	c.JSON(200, gin.H{"message": "帳號新增成功"})
}

// 查詢(帳號登入) ver.1完成
func Member_Query(c *gin.Context) {

	// 變數
	SearchInfo := NumberStruct{}           // 使用者傳送的帳號和密碼
	checkInfo := NumberStruct{}            // 接收資料庫傳回的密碼
	SaltStruct := SaltStruct{}             // 接收資料庫傳回的鹽值
	SearchResponse := AccountLoginReturn{} // 用於回傳用戶
	tx := database.MyConnect.Begin()       // 紀錄資料庫狀態
	var count int64
	var checkHash string
	var err error

	// 設定回應
	SearchResponse.State = "0"
	SearchResponse.AdministratorState = "0"
	SearchResponse.LoginState = "0"

	// 接收參數
	if err := c.ShouldBindJSON(&SearchInfo); err != nil {

		// 參數
		SearchResponse.State = "1" // 有錯誤
		SearchResponse.Message = "綁定失敗"

		// 回應
		c.JSON(http.StatusBadGateway, SearchResponse)
		log.Printf("綁定失敗 原因:%S", err)
		return
	}

	// 檢查結構
	if err = Account_Information_Check(SearchInfo, 2, c.ClientIP()); err != nil {

		// 參數
		SearchResponse.State = "1" // 有錯誤
		SearchResponse.Message = err.Error()

		// 錯誤
		c.JSON(400, gin.H{"message": SearchResponse})
		log.Print(err)
		return
	}

	// 調用資料庫 取回hash過的密碼
	if err = database.MyConnect.Table("rip_current_member").Select("memberPassword").Where("memberGmail = ?", SearchInfo.MemberGmail).Count(&count).Find(&checkInfo).Error; err != nil {

		// 參數
		SearchResponse.State = "1" // 有錯誤
		SearchResponse.Message = "資料庫異常"

		// 回應
		c.JSON(http.StatusInternalServerError, gin.H{"message": SearchResponse})
		log.Printf("取回密碼失敗 原因:%v", err)
		tx.Rollback()
		return
	}

	// 調用資料庫 取回密碼的鹽值
	if err = database.MyConnect.Table("rip_current_salt").Select("passwordSalt").Where("userID = ?", SearchInfo.MemberGmail).Find(&SaltStruct).Error; err != nil {

		// 參數
		SearchResponse.State = "1" // 有錯誤
		SearchResponse.Message = "資料庫異常"

		// 回應
		c.JSON(http.StatusInternalServerError, gin.H{"message": SearchResponse})
		log.Printf("取回鹽值失敗 原因:%v", err)
		tx.Rollback()
		return
	}

	// 產生雜湊 用於驗證
	if checkHash, err = PasswordCheck(SearchInfo.MemberPassword, SaltStruct.PasswordSalt); err != nil {

		// 參數
		SearchResponse.State = "1" // 有錯誤
		SearchResponse.Message = "資料庫異常"

		// 回應
		c.JSON(400, gin.H{"message": SearchResponse})
		log.Printf("產成雜湊失敗 原因:%v", err)
		return
	}

	// 驗證密碼是否相同
	if count > 0 { // 帳號存在

		// 帳號存在 檢查密碼是否相同
		if checkInfo.MemberPassword == checkHash {

			// 檢查是不是管理員
			if err := database.MyConnect.Table("rip_current_administrator_permissions").Where("userID = ?", SearchInfo.MemberGmail).Count(&count).Error; err != nil {

				// 錯誤
				log.Printf("檢查是否為管理員失敗 原因:%v", err)
			} else {

				//
				if count > 0 {

					// 參數
					SearchResponse.AdministratorState = "1" // 是管理員
					SearchResponse.LoginState = "1"         // 登入成功
					SearchResponse.State = "0"              // 沒錯誤
					SearchResponse.Message = "登入成功"
				} else {

					// 設定參數
					SearchResponse.AdministratorState = "0" // 不是管理員
					SearchResponse.LoginState = "1"         // 登入成功
					SearchResponse.State = "0"              // 沒錯誤
					SearchResponse.Message = "登入成功"
				}
			}

			// 成功 目標存在
			log.Printf("帳號:%s 密碼正確", SearchInfo.MemberGmail)
			c.JSON(http.StatusOK, gin.H{"message": SearchResponse})
		} else {

			// 設定參數
			SearchResponse.AdministratorState = "0" // 不是管理員
			SearchResponse.LoginState = "0"         // 登入失敗
			SearchResponse.State = "3"              // 密碼錯誤
			SearchResponse.Message = "密碼錯誤"

			// 失敗 目標不存在
			log.Printf("帳號:%s 密碼錯誤", SearchInfo.MemberGmail)
			c.JSON(http.StatusUnauthorized, gin.H{"message": SearchResponse})
		}
	} else { // 帳號錯誤

		// 設定參數
		SearchResponse.AdministratorState = "0" // 不是管理員
		SearchResponse.LoginState = "0"         // 登入失敗
		SearchResponse.State = "1"              // 帳號錯誤
		SearchResponse.Message = "帳號不存在"

		// 帳號錯誤
		log.Printf("帳號:%s 帳號不存在", SearchInfo.MemberGmail)
		c.JSON(http.StatusNotFound, gin.H{"message": SearchResponse})
	}
}

// 查詢(帳號是否存在)
func Member_Query_Exist(c *gin.Context) {

	// 變數
	checkInfo := NumberStruct{}
	var count int64

	// 結合
	if err := c.ShouldBindJSON(&checkInfo); err != nil {
		c.JSON(400, gin.H{"message": "綁定失敗"})
		log.Printf("綁定失敗 原因:%S", err)
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
		c.JSON(400, gin.H{"message": "綁定失敗"})
		log.Printf("綁定失敗 原因:%S", err)
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

// 修改用戶資料 ver.1完成
func Member_Modify_Other(c *gin.Context) {

	// 修改帳號資料 目前允許:帳號名稱

	// 變數
	ModifyInfo := NumberStruct{}

	// 接收參數
	if err := c.ShouldBindJSON(&ModifyInfo); err != nil {
		c.JSON(400, gin.H{"message": "綁定失敗"})
		log.Printf("綁定失敗 原因:%S", err)
		return
	}

	// 允許修改的欄位
	updateFields := map[string]interface{}{
		"memberName": ModifyInfo.MemberName,
	}

	// // 修改
	if err := database.MyConnect.Table("rip_current_member").Where("memberGmail = ?", ModifyInfo.MemberGmail).Updates(updateFields).Error; err != nil {
		c.JSON(400, gin.H{"message": "修改失敗"})
		log.Printf("帳號:%s修改資訊失敗 原因:%s", ModifyInfo.MemberGmail, err)
		return
	}

	// // 結果
	log.Printf("帳號:%s 修改資訊成功", ModifyInfo.MemberGmail)
	c.JSON(200, gin.H{"message": "修改資訊成功"})
}

// 修改密碼ver.1完成
func Member_Modify_Password(c *gin.Context) {

	//簡介:

	//變數
	UserAccount := ModifyPassword{} // 用於確認帳號是否正確
	CheckPassword := ""
	CheckSalt := ""
	var NewPassword string
	var NewSalt string
	var err error
	tx := database.MyConnect.Begin() // 紀錄資料庫狀態

	// 綁定
	if err := c.ShouldBindJSON(&UserAccount); err != nil {
		c.JSON(400, gin.H{"message": "綁定失敗"})
		log.Printf("綁定失敗 原因:%S", err)
		return
	}

	// 資料預處理
	UserAccount.UserID = strings.ReplaceAll(UserAccount.UserID, " ", "")
	UserAccount.UserPassword = strings.ReplaceAll(UserAccount.UserPassword, " ", "")
	UserAccount.NewPassword = strings.ReplaceAll(UserAccount.NewPassword, " ", "")

	log.Printf("測試:%v", UserAccount.UserID)

	// 驗證帳號是對的
	if err := database.MyConnect.Table("rip_current_member").Where("memberGmail = ?", UserAccount.UserID).Select("memberPassword").Find(&CheckPassword).Error; err != nil {

		//
		log.Printf("失敗%v", err)
		c.JSON(400, "")
	}

	if len(strings.ReplaceAll(CheckPassword, " ", "")) != 0 {

		// 有密碼 可繼續
		if err := database.MyConnect.Table("rip_current_salt").Where("userID = ?", UserAccount.UserID).Select("passwordSalt").Find(&CheckSalt).Error; err != nil {

			// 錯誤
		}

		// 產生Hash
		if UserAccount.UserPassword, err = PasswordCheck(UserAccount.UserPassword, CheckSalt); err != nil {

			// 發生錯誤
		}

		// 進行比較
		if UserAccount.UserPassword != CheckPassword {

			// 錯誤 密碼並不相符

		}

	} else {

		// 錯誤 密碼不存在
		log.Printf("帳號:%v 不存在", UserAccount.UserID)
		c.JSON(400, gin.H{"message": "帳號或密碼錯誤"})
		return
	}

	// 產生新的密碼
	if NewPassword, NewSalt, err = HashArgon2(UserAccount.NewPassword); err != nil {

		// 錯誤
		c.JSON(400, gin.H{"message": "修改密碼失敗"})
		log.Printf("產生新密碼失敗 原因:%v", err)
	}

	// 將新密碼傳送到資料庫
	if err := database.MyConnect.Table("rip_current_member").Where("memberGmail = ?", UserAccount.UserID).Updates(map[string]interface{}{"memberPassword": NewPassword}).Error; err != nil {

		// 錯誤
		c.JSON(400, gin.H{"message": "修改密碼失敗"})
		log.Printf("更新密碼失敗 原因:%v", err)
		tx.Rollback()
		return
	}

	// 將新密碼的鹽值傳送到資料庫
	if err := database.MyConnect.Table("rip_current_salt").Where("userID = ?", UserAccount.UserID).Updates(map[string]interface{}{"passwordSalt": NewSalt}).Error; err != nil {

		// 錯誤
		c.JSON(400, gin.H{"message": "修改密碼失敗"})
		log.Printf("更新鹽值失敗 原因:%v", err)
		tx.Rollback()
		return
	}

	// 成功
	c.JSON(200, gin.H{"message": "更新密碼成功"})
}

// 忘記密碼-前置處理 #需要內容看裡面
func Member_Modify_PasswordForgot_check(c *gin.Context) {

	// 調用方法
	if c.Request.Method == "GET" {

		// 簡介: 需要帳號

		// 變數
		Target := NumberStruct{}
		TargetResponse := PasswordForgetResponse{}
		Target.MemberGmail = c.DefaultQuery("gmail", "")
		Target.MemberGmail = strings.ReplaceAll(Target.MemberGmail, " ", "")

		// 檢查
		if Target.MemberGmail == "" {
			c.JSON(400, gin.H{"message": "帳號不可為空"})
			log.Print("查詢參數帳號不可為空")
			return
		}

		// 發送驗證碼
		if mod, err := Member_OTP_Create(Target); err != nil {

			// 訊息
			TargetResponse.Message = err.Error()
			TargetResponse.State = mod

			// 錯誤
			if mod == "1" {

				// 錯誤
				c.JSON(401, gin.H{"message": TargetResponse})
				log.Printf("OTP產生失敗 原因:%v", err)
				return
			} else {

				// 錯誤
				c.JSON(400, gin.H{"message": TargetResponse})
				log.Printf("重複請求 原因:%v", err)
				return
			}
		}

		// 訊息
		TargetResponse.Message = "USER OTP已經產生"
		TargetResponse.State = "0"

		// 成功
		c.JSON(200, gin.H{"message": TargetResponse})

	} else if c.Request.Method == "POST" {

		// 簡介: 需要提供帳號、驗證碼

		// 變數
		Target := UserOTPstruct{} // 主要為用戶驗證碼 UserOTPstruct
		passcode := make([]byte, 16/2)

		// 接收參數
		if err := c.ShouldBindJSON(&Target); err != nil {
			c.JSON(400, gin.H{"message": PasswordForgetResponse{Message: "綁定失敗", State: "1"}})
			log.Printf("綁定失敗 原因:%S", err)
			return
		}

		// 取回金鑰
		if value, ok := templates.UserOTPCache.Load(Target.UserID); ok {

			// 斷言
			if OTPsecret, ok := value.(UserOTPstruct); !ok {

				// 錯誤
				c.JSON(400, gin.H{"message": PasswordForgetResponse{Message: "驗證失敗", State: "1"}})
				log.Print("取值失敗")
				return

			} else {

				// 賦值
				Target.UserOTPsecret = OTPsecret.UserOTPsecret
			}
		}

		// 驗證
		if isVaild := totp.Validate(Target.VerificationCode, Target.UserOTPsecret); !isVaild {

			// 錯誤 不相同
			c.JSON(400, gin.H{"message": PasswordForgetResponse{Message: "驗證失敗", State: "1"}})
			log.Printf("用戶:%v 驗證失敗", Target.UserID)
			return

		} else {

			// 進入修改程序

			// #建立一組暫時的修改通行碼
			if _, err := rand.Read(passcode); err != nil {

				// 發生錯誤
				c.JSON(400, gin.H{"message": PasswordForgetResponse{Message: "驗證成功 但發生錯誤", State: "1"}})
				log.Printf("用戶:%v 驗證成功 但發生錯誤 原因:%v", Target.UserID, err)
				return
			}

			log.Printf("passcode0:%v", passcode)
			log.Printf("passcode1:%v", string(passcode))

			// #存放
			templates.UserOTPCache.Store(base64.StdEncoding.EncodeToString(passcode), UserOTPstruct{UserID: Target.UserID, InsertTime: time.Now()})

			log.Printf("passcode2:%v", string(passcode))

			// #回傳客戶端
			c.JSON(200, gin.H{"message": PasswordForgetResponse{Message: base64.StdEncoding.EncodeToString(passcode), State: "1"}})
		}

	} else {

		// 錯誤
		c.JSON(400, gin.H{"message": "只有POST和GET"})
		return
	}
}

// 忘記密碼-正式執行 #需要通行碼、新密碼
func Member_Modify_PasswordForgot_Reset(c *gin.Context) {

	// 變數
	var Target UserNewPasswordstruct

	// 接收參數
	if err := c.ShouldBindJSON(&Target); err != nil {
		c.JSON(400, gin.H{"message": "綁定失敗"})
		log.Printf("綁定失敗 原因:%S", err)
		return
	}

	//
	log.Printf("測試:%v", Target)

	// 根據驗證碼 取回帳號來修改密碼
	if value, ok := templates.UserOTPCache.Load(Target.Passcode); ok {

		log.Printf("passcode:%v", Target.Passcode)

		// 斷言
		if userID, ok := value.(UserOTPstruct); ok {

			// 開始操作吧 #要求新密碼、確認密碼 #不需要驗證帳號的真實性

			// 變數
			var NewHashPassword string
			var NewSalt string
			var err error

			// #確認格式
			if err := Account_Information_Check(Target, 3, ""); err != nil {

				// 錯誤
				c.JSON(400, gin.H{"message": err})
				log.Printf("忘記密碼修改密碼失敗 原因:%v", err)
				return
			}

			// 產生新的密碼
			if NewHashPassword, NewSalt, err = HashArgon2(Target.NewPassword); err != nil {

				// 錯誤
				c.JSON(400, gin.H{"message": "修改密碼失敗"})
				log.Printf("產生新密碼失敗 原因:%v", err)
				return
			}

			// 調用資料庫
			if err := database.MyConnect.Table("rip_current_member").Where("memberGmail = ?", userID.UserID).Updates(map[string]interface{}{"memberPassword": NewHashPassword}).Error; err != nil {

				// 錯誤
				c.JSON(400, gin.H{"message": fmt.Sprintf("帳號:%v 忘記密碼修改失敗", userID.UserID)})
				log.Printf("帳號:%v 忘記密碼調用資料庫失敗 原因:%v", userID, err)
				return
			}

			// 更新鹽值
			if err := database.MyConnect.Table("rip_current_salt").Where("userID = ?", userID.UserID).Updates(map[string]interface{}{"passwordSalt": NewSalt}).Error; err != nil {

				// 錯誤
				c.JSON(400, gin.H{"message": fmt.Sprintf("帳號:%v 忘記密碼修改失敗", userID.UserID)})
				log.Printf("帳號:%v 忘記密碼調用資料庫失敗 原因:%v", userID, err)
				return
			}

			// 成功
			c.JSON(200, gin.H{"message": "忘記密碼修改成功"})
			log.Printf("用戶:%v 忘記密碼更新成功", userID.UserID)

			// 移除通行證
			templates.UserOTPCache.Delete(Target.Passcode)
			log.Printf("清掉用戶:%v的通行證 在時間:%v", userID.UserID, time.Now().Format("2006-01-02 15:04:05"))
		} else {

			// 失敗
			c.JSON(400, gin.H{"message": "錯誤"})
			log.Print("斷言失敗")
		}

	} else {

		log.Printf("passcode:%v", Target.Passcode)

		// 目標
		c.JSON(400, gin.H{"message": "通行碼不存在"})
	}
}

// 通用方法-----------------------------------------------------------------------------------------------------------------------------------------

// 對傳過來的帳號資訊做檢查
func Account_Information_Check(target interface{}, Mod int16, Ip string) error {

	// 簡介:不希望有空值 不希望電話號碼不符合格式

	//變數

	switch Mod {
	case 1: // 模式1:帳號註冊

		// 簡介: 需要帳號、密碼、電話

		// 先斷言
		if value, ok := target.(NumberStruct); ok {
			//變數
			value.MemberGmail = strings.ReplaceAll(value.MemberGmail, " ", "")
			value.MemberPassword = strings.ReplaceAll(value.MemberPassword, " ", "")
			value.MemberPhone = strings.ReplaceAll(value.MemberPhone, " ", "")
			var errString string = ""
			var count int64

			// 帳號檢查
			if value.MemberGmail == "" {

				// 錯誤
				errString += "Gmail為空值 "
				log.Printf("%v 觸發了", value.MemberGmail)
				//return fmt.Errorf(errString)
			} else {

				// 檢查是否已存在
				if err := database.MyConnect.Table("rip_current_member").Where("memberGmail = ?", value.MemberGmail).Count(&count).Error; err != nil {

					// 錯誤
					log.Printf("檢查電子郵件 調用資料庫失敗 原因:%v", err)
					return errors.New("資料庫異常")
				} else {

					// 判斷是否存在
					if count != 0 {
						return errors.New("電子郵件已存在")
					} else {

						// 檢查電子郵件是否為真
						// if err := GmailCheck(value.MemberGmail, Ip); err != nil {

						// 	// 錯誤
						// 	return err
						// }
					}
				}
			}

			// 歸零
			count = 0

			// 密碼檢查
			if value.MemberGmail == "" {

				// 錯誤
				errString += "密碼為空值 "
				log.Printf("%v 觸發了", value.MemberGmail)
				//return fmt.Errorf("密碼為空值")
			}

			// 電話檢查
			if len(value.MemberPhone) != 10 {

				// 錯誤
				errString += "電話不符合格式 "
				log.Printf("%v 觸發了", value.MemberGmail)
				//return fmt.Errorf("電話不符合格式")
			} else {

				if err := database.MyConnect.Table("rip_current_member").Where("memberPhone = ?", value.MemberPhone).Count(&count).Error; err != nil {

					// 錯誤
					log.Printf("檢查電話 調用資料庫失敗 原因:%v", err)
					return errors.New("資料庫異常")
				} else {

					if count != 0 {
						return errors.New("電話已存在")
					}
				}
			}

			if errString != "" {

				// 有錯誤
				return errors.New(errString)
			}
		}
	case 2: // 模式2:帳號登入

		// 簡介: 需要帳號、密碼

		if value, ok := target.(NumberStruct); ok {

			//變數
			value.MemberGmail = strings.ReplaceAll(value.MemberGmail, " ", "")
			value.MemberPassword = strings.ReplaceAll(value.MemberPassword, " ", "")
			var errString string = ""

			// 帳號檢查
			if value.MemberGmail == "" {

				// 錯誤
				errString += "Gmail為空值 "
				log.Printf("%v 觸發了", value.MemberGmail)
				//return fmt.Errorf(errString)
			}

			// 密碼檢查
			if value.MemberGmail == "" {

				// 錯誤
				errString += "密碼為空值 "
				log.Printf("%v 觸發了", value.MemberGmail)
				//return fmt.Errorf("密碼為空值")
			}

			if errString != "" {

				// 有錯誤
				return errors.New(errString)
			}
		}
	case 3: // 密碼格式去認

		if value, ok := target.(UserNewPasswordstruct); ok {

			// 變數
			value.NewPassword = strings.ReplaceAll(value.NewPassword, " ", "")

			if value.NewPassword == "" {

				return errors.New("密碼為空")
			}
		}
	}

	return nil
}

// 用於註冊時() 回傳Base64的哈希密碼,Base64的鹽值
func HashArgon2(password string) (string, string, error) {

	// 變數
	var salt []byte
	var err error

	// 參數
	if salt, err = GenerateRandomSalt(16); err != nil {

		// 產生鹽值時發生錯誤
		return "", "", err
	}

	timeCost := uint32(1)
	memoryCost := uint32(64 * 1024)
	threads := uint8(4)
	keyLength := uint32(32)

	// 哈希密碼
	hash := argon2.IDKey([]byte(password), salt, timeCost, memoryCost, threads, keyLength)
	return base64.StdEncoding.EncodeToString(hash), base64.StdEncoding.EncodeToString(salt), nil
}

// 用於驗證(原始密碼,Base64的Salt) 回傳Base64的Hash
func PasswordCheck(password string, salt string) (string, error) {

	// 變數
	var DecodeSalt []byte
	var err error

	// 用於Argon2的參數
	timeCost := uint32(1)
	memoryCost := uint32(64 * 1024)
	threads := uint8(4)
	keyLength := uint32(32)

	// 將以Base64存放到資料庫的鹽值轉回成[]byte
	if DecodeSalt, err = base64.StdEncoding.DecodeString(salt); err != nil {

		// 錯誤
		return "", err
	}

	// 哈希
	hash := argon2.IDKey([]byte(password), DecodeSalt, timeCost, memoryCost, threads, keyLength)
	return base64.StdEncoding.EncodeToString(hash), nil
}

// 產生鹽值 可能需要Base64
func GenerateRandomSalt(length int) ([]byte, error) {

	// 變數
	salt := make([]byte, length)
	var length_check int
	var err error

	// 產生鹽值
	if length_check, err = rand.Read(salt); err != nil {

		// 錯誤
		return salt, err
	}

	// 成功
	log.Printf("檢查鹽值長度:%v", length_check)
	return salt, nil
}

// 驗證Gmail
func GmailCheck(gmail string, ip string) error {

	// 變數
	zerobouncego.API_KEY = "1e9271766cba4b6aa1894007c4c2ab66"

	// 請求開始
	response, error_ := zerobouncego.Validate(gmail, ip)

	if error_ != nil {

		// 發生錯誤
		log.Printf("驗證電子郵件發生錯誤 原因:%v ", error_.Error())
		return errors.New("驗證電子郵件時發生錯誤")

	} else {

		if response.Status == zerobouncego.S_VALID {

			// 又效
			return nil
		} else {

			// 無效
			return fmt.Errorf("目標:%v 是無效的", gmail)
		}
	}
}

// 建立短暫(30秒)的OTP驗證 提供管理員做驗證 #Permissions、Password、Salt #包含Gmail傳送
func Member_OTP_Create(OTPsecret NumberStruct) (string, error) {

	// 變數
	var TOTPsecret string       // 金鑰
	var otp string              // 驗證碼
	var operation UserOTPstruct // 要存放的結構

	// 取回金鑰 #目標不存在就取不回 不用做帳號驗證
	if err := database.MyConnect.Table("rip_current_member").Where("memberGmail = ?", OTPsecret.MemberGmail).Select("userOTPSecret").Find(&TOTPsecret).Error; err != nil {

		// 錯誤
		return "1", fmt.Errorf("取回TOTP 失敗%v", err)

	} else {

		// 確保同時只有一筆紀錄 免得撞車
		if value, ok := templates.UserOTPCache.Load(OTPsecret.MemberGmail); ok {

			// 斷言
			if value, ok := value.(UserOTPstruct); ok {

				// 變數 產生時間
				remain := time.Since(value.InsertTime).Seconds()
				var remaining int

				// 檢查是否超過 30 秒
				if remain < 30 {

					// 小於30秒
					remaining = 30 - int(remain) // 計算剩餘秒數，並取整數
					log.Printf("還剩下 %d 秒\n", remaining)
					return "2", fmt.Errorf("用戶:%s 已經發送請求 請:%d後在試", OTPsecret.MemberGmail, remaining)

				} else {

					// 已經超過 把數據刪掉
					templates.UserOTPCache.Delete(OTPsecret.MemberGmail)
					log.Printf("清掉管理者:%v 在時間:%v 驗證碼為:%v", OTPsecret.MemberGmail, time.Now().Format("2006-01-02 15:04:05"), value.VerificationCode)
				}
			}
		}

		// 確保OTPsecret不為空
		if TOTPsecret == "" {

			// 輸出
			log.Printf("用戶:%v 的OTPsecret為空", OTPsecret.MemberGmail)
			return "1", errors.New("操作失敗 伺服器問題")
		} else {

			// 輸出
			log.Printf("用戶:%v 的OTPsecret為:%v", OTPsecret.MemberGmail, TOTPsecret)
		}

		// 產生驗證碼
		if otp, err = totp.GenerateCode(TOTPsecret, time.Now()); err != nil {

			// 錯誤
			return "1", fmt.Errorf("產生驗證碼失敗 原因:%v", err)
		}

		// 修改
		operation.UserID = OTPsecret.MemberGmail // 帳號名稱
		operation.UserOTPsecret = TOTPsecret     // 金鑰
		operation.InsertTime = time.Now()        // 新增時間
		operation.VerificationCode = otp         // 驗證碼

		// 存放
		templates.UserOTPCache.Store(OTPsecret.MemberGmail, operation)

		// 發送給客戶端
		SendGmail(operation.UserID, otp, "1")
		log.Printf("驗證碼:%v", otp)
	}

	// 回傳
	return "0", nil
}

// 觸發更新每30秒一次
func Member_ScheduleOTPUpdates() {

	for {
		time.Sleep(30 * time.Second)
		Member_CleanOTPRecode()
	}
}

// 清除OTP紀錄
func Member_CleanOTPRecode() {

	// 變數
	now := time.Now()
	var count int64

	// 計算
	templates.UserOTPCache.Range(func(key, value any) bool {

		// 取值
		if _, ok := value.(UserOTPstruct); !ok {

			// 錯誤
			return true

		} else {

			//
			count++
			return true
		}
	})

	if count != 0 {
		log.Printf("用戶OTP數據總量:%v", count)
	}

	// 開始操作
	templates.UserOTPCache.Range(func(key, value any) bool {

		// 取值
		if checkTime, ok := value.(UserOTPstruct); !ok {

			// 錯誤
			return true

		} else {

			// 成功 來驗證
			if now.Sub(checkTime.InsertTime) > 30*time.Second {
				templates.UserOTPCache.Delete(checkTime.UserID)
				log.Printf("清掉管理者:%v 在時間:%v 驗證碼為:%v", checkTime.UserID, time.Now().Format("2006-01-02 15:04:05"), checkTime.VerificationCode)
			}
			return true
		}
	})

	count = 0

	templates.UserOTPCache.Range(func(key, value any) bool {

		// 取值
		if test, ok := value.(UserOTPstruct); !ok {

			log.Printf("測試:%v", test)

		}

		count++
		return true
	})

	// 輸出
	if count != 0 {
		log.Printf("用戶OTP剩餘數據總量:%v", count)
	}
}
