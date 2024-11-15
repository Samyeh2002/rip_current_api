package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"rip_current_mod/database"
	"rip_current_mod/templates"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"gopkg.in/gomail.v2"
	"gorm.io/gorm"
)

// 簡介:處理管理者相關的功能 和Member有一定的牽連

// 結構-管理者權限
type Permission struct {
	UserID                     string `gorm:"column:userID"`                     // 管理者名單
	SuspendedPermissions       string `gorm:"column:suspendedPermissions"`       //能否控制帳戶停權
	PasswordPenaltyPermissions string `gorm:"column:passwordPenaltyPermissions"` // 解除用戶的密碼處罰
	PhotoSuspensionPermissions string `gorm:"column:photoSuspensionPermissions"` // 限制用戶能否上傳圖片
	AccountDeletePermissions   string `gorm:"column:accountDeletePermissions"`   // 刪除用戶帳號
	OperationTarget            string `gorm:"-"`                                 // 要操作的目標用戶
	UserOTPSecret              string `gorm:"column:userOTPSecret"`
}

type OTP struct {
	UserID           string `gorm:"column:userID"`
	VerificationCode string `gorm:"column:userID"` // 存放OTPsecret
	InsertTime       time.Time
	other            Permission
}

// 給用戶用
type UserOTP struct {
	UserID           string // 用戶ID
	VerificationCode string // 驗證碼
}

// 結構-管理者權限的鹽值
type PermissionSalt struct {
	UserID                         string `gorm:"column:userID"`                         // 管理者名單
	SuspendedPermissionsSalt       string `gorm:"column:suspendedPermissionsSalt"`       //能否控制帳戶停權
	PasswordPenaltyPermissionsSalt string `gorm:"column:passwordPenaltyPermissionsSalt"` // 解除用戶的密碼處罰
	PhotoSuspensionPermissionsSalt string `gorm:"column:photoSuspensionPermissionsSalt"` // 限制用戶能否上傳圖片
	AccountDeletePermissionsSalt   string `gorm:"column:accountDeletePermissionsSalt"`   // 刪除用戶帳號
}

// 結構-管理者權限的哈希值
type PermissionHash struct {
	UserID                         string `gorm:"column:userID"`                         // 管理者名單
	SuspendedPermissionsHash       string `gorm:"column:suspendedPermissionsHash"`       //能否控制帳戶停權
	PasswordPenaltyPermissionsHash string `gorm:"column:passwordPenaltyPermissionsHash"` // 解除用戶的密碼處罰
	PhotoSuspensionPermissionsHash string `gorm:"column:photoSuspensionPermissionsHash"` // 限制用戶能否上傳圖片
	AccountDeletePermissionsHash   string `gorm:"column:accountDeletePermissionsHash"`   // 刪除用戶帳號
}

// 管理員基本操作-新增管理者
func InsertAdministrator(c *gin.Context) {

	// 變數
	target := Permission{}
	tx := database.MyConnect.Begin()
	var count int64

	log.Printf("近來囉")

	// 綁定資料
	if err := c.ShouldBindJSON(&target); err != nil {

		// 錯誤
		c.JSON(400, gin.H{"message": "綁定失敗"})
		log.Printf("綁定失敗 原因:%v", err)
		return
	}

	// 確認該帳號存在
	if err := tx.Table("rip_current_member").Where("memberGmail = ?", target.UserID).Count(&count).Error; err != nil {

		// 錯誤
		c.JSON(400, gin.H{"message": "新增管理員失敗"})
		log.Printf("資料庫操作失敗 原因:%v", err)
		return
	} else {

		if count == 0 {

			// 帳號不存在
			c.JSON(400, gin.H{"message": "帳號不存在"})
			log.Printf("帳號不存在 原因:%v", err)
			return
		}
	}

	// 修改member的is_administrator
	if err := tx.Table("rip_current_member").Where("memberGmail = ?", target.UserID).Updates(map[string]interface{}{"is_administrator": "1"}).Error; err != nil {

		// 錯誤
		c.JSON(400, gin.H{"message": "新增管理員失敗"})
		log.Printf("修改is_administrator失敗 原因:%v", err)
		return
	}

	// 生成一個新的TOTP秘密（類似於用戶註冊時生成）
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Alderonius",
		AccountName: "Alderonius2002@gmail.com",
	})

	// 發生錯誤
	if err != nil {

		c.JSON(200, gin.H{"message": "新增管理員失敗"})
		log.Print("Error generating key:", err)
		return
	}

	// 添加
	target.UserOTPSecret = key.Secret()

	// 新增到資料庫 明碼表
	if err := tx.Table("rip_current_administrator_permissions").Create(&target).Error; err != nil {

		// 錯誤
		c.JSON(200, gin.H{"message": "新增管理員失敗"})
		log.Printf("資料庫操作失敗 原因:%v", err)
		tx.Rollback()
		return
	}

	// 新增到資料庫 鹽值表
	if err := tx.Table("rip_current_administrator_salt").Create(&PermissionSalt{UserID: target.UserID}).Error; err != nil {

		// 錯誤
		c.JSON(200, gin.H{"message": "新增管理員失敗"})
		log.Printf("資料庫操作失敗 原因:%v", err)
		tx.Rollback()
		return
	}

	// 新增到資料庫 雜湊表
	if err := tx.Table("rip_current_administrator_Hash").Create(&PermissionHash{UserID: target.UserID}).Error; err != nil {

		// 錯誤
		c.JSON(200, gin.H{"message": "新增管理員失敗"})
		log.Printf("資料庫操作失敗 原因:%v", err)
		tx.Rollback()
		return
	}

	// 賦予金鑰
	if _, err = UpdatePermissions(2, target, tx); err != nil {

		// 錯誤
		c.JSON(200, gin.H{"message": "新增管理員失敗"})
		log.Printf("賦予金鑰失敗 原因:%v", err)
		tx.Rollback()
		return
	}

	// 新增成功
	c.JSON(200, gin.H{"message": "新增成功"})
	log.Printf("帳號:%v 管理員身分新增成功", target.UserID)
	tx.Commit()
}

// 管理員基本操作-刪除管理者
func DeleteAdministrator(c *gin.Context) {

	// 變數
	target := Permission{}
	tx := database.MyConnect.Begin()
	var count int64

	log.Printf("出去囉")

	// 綁定資料
	if err := c.ShouldBindJSON(&target); err != nil {

		// 錯誤
		c.JSON(400, gin.H{"message": "綁定失敗"})
		log.Printf("綁定失敗 原因:%v", err)
		return
	}

	// 檢查帳號是否存在
	if err := tx.Table("rip_current_administrator_permissions").Where("userID = ?", target.UserID).Count(&count).Error; err != nil {

		// 錯誤
		c.JSON(400, gin.H{"message": "操作失敗"})
		log.Printf("資料庫操作失敗1 原因:%v", err)
		tx.Rollback()
		return

	} else {

		if count == 0 {

			// 不是管理員
			c.JSON(400, gin.H{"message": "該帳號不是管理員"})
			log.Printf("帳號:%v 不是管理員", target.UserID)
			return
		}
	}

	// 刪除帳號
	if err := tx.Table("rip_current_administrator_permissions").Where("userID = ?", target.UserID).Delete(&Permission{}).Error; err != nil {

		// 錯誤
		c.JSON(400, gin.H{"message": "刪除管理員失敗"})
		log.Printf("刪除管理員操作失敗 原因:%v", err)
		tx.Rollback()
		return
	}

	// 修改member的is_administrator
	if err := tx.Table("rip_current_member").Where("memberGmail = ?", target.UserID).Updates(map[string]interface{}{"is_administrator": "0"}).Error; err != nil {

		// 錯誤
		c.JSON(400, gin.H{"message": "刪除管理員失敗"})
		log.Printf("修改is_administrator失敗 原因:%v", err)
		tx.Rollback()
		return
	}

	// 成功
	c.JSON(200, gin.H{"message": "帳號刪除成功"})
	log.Printf("帳號:%v 管理員身分刪除成功", target.UserID)
	tx.Commit()
}

// 管理員權限操作 (理論上要有token和OTP) 步驟1:使用Permission(帳號和指令) 並發送OTP
func Permissions_Operation(c *gin.Context) {

	// 變數
	Target := Permission{}
	PermissionChangeFail := ""
	var count int64

	// 進行綁定
	if err := c.ShouldBindJSON(&Target); err != nil {

		// 錯誤
		c.JSON(400, gin.H{"message": "綁定失敗"})
		log.Printf("綁定失敗 原因:%S", err)
		return
	}

	// 確認管理員帳號存在
	if err := database.MyConnect.Table("rip_current_administrator_permissions").Where("userID = ?", Target.UserID).Count(&count).Error; err != nil {

		// 錯誤
		c.JSON(400, gin.H{"message": "權限操作失敗"})
		log.Printf("資料庫操作失敗 原因:%S", err)
		return

	} else {

		if count == 0 {

			// 帳號不存在
			c.JSON(400, gin.H{"message": "管理員帳號不存在"})
			log.Printf("帳號不存在 原因:%S", err)
			return
		}

		// count歸零
		count = 0
	}

	// 確認使用者帳號存在
	if err := database.MyConnect.Table("rip_current_member").Where("memberGmail = ?", Target.OperationTarget).Count(&count).Error; err != nil {

		// 錯誤
		c.JSON(400, gin.H{"message": "權限操作失敗"})
		log.Printf("資料庫操作失敗 原因:%S", err)
		return

	} else {

		if count == 0 {

			// 帳號不存在
			c.JSON(400, gin.H{"message": "使用者帳號不存在"})
			log.Printf("帳號不存在 原因:%S", err)
			return
		}
	}

	// 進行權限認證: 用戶停權 #尚未測試
	if Target.AccountDeletePermissions == "1" {

		// 變數
		PermissionsName := "suspendedPermissions"
		var password string
		var salt string
		var hash string

		log.Printf("觸發權限:%v的操作", PermissionsName)

		// 驗證開始
		if err := database.MyConnect.Table("rip_current_administrator_permissions").Where("userID = ?", Target.UserID).Select("suspendedPermissions").Find(&password).Error; err != nil {

			// 調用資料庫發生錯誤 明碼
			log.Printf("進行權限:%v 操作失敗 原因:%v", PermissionsName, err)
			PermissionChangeFail += " 帳號停權失敗 "
		} else {

			// Salt
			if err := database.MyConnect.Table("rip_current_administrator_salt").Where("userID = ?", Target.UserID).Select("suspendedPermissionsSalt").Find(&salt).Error; err != nil {

				// 調用資料庫發生錯誤 鹽值
				log.Printf("進行權限:%v 操作失敗 原因:%v", PermissionsName, err)
				PermissionChangeFail += " 帳號停權失敗 "
			} else {

				// Hash
				if err := database.MyConnect.Table("rip_current_administrator_Hash").Where("userID = ?", Target.UserID).Select("suspendedPermissionsHash").Find(&hash).Error; err != nil {

					// 調用資料庫發生錯誤 哈希值
					log.Printf("進行權限:%v 操作失敗 原因:%v", PermissionsName, err)
					PermissionChangeFail += " 帳號停權失敗 "
				} else {

					// 三種值都取回了 驗證吧
					if checkPassword, err := PasswordCheck(password, salt); err != nil {

						// 發生錯誤
						log.Printf("進行權限:%v 操作失敗 原因:%v", PermissionsName, err)
						PermissionChangeFail += " 帳號停權失敗 "
					} else {

						if hash != checkPassword {

							// 不符合 禁止權限操作
							log.Printf("進行權限:%v 操作失敗 原因:%v", PermissionsName, errors.New("Hash並不相符不允許操作"))
							PermissionChangeFail += " 沒有權限帳號停權失敗 "
						} else {

							// 禁止帳號登入吧
						}
					}
				}
			}
		}
	}

	// 進行權限認證: 解除用戶密碼處罰

	// 進行權限認證: 禁止用戶上傳圖片

	// 進行權限認證: 刪除用戶帳號
	if Target.AccountDeletePermissions == "1" {

		// 變數
		PermissionsName := "accountDeletePermissions"
		var password string
		var salt string
		var hash string

		// 驗證開始
		if err := database.MyConnect.Table("rip_current_administrator_permissions").Where("userID = ?", Target.UserID).Select("accountDeletePermissions").Find(&password).Error; err != nil {

			// 調用資料庫發生錯誤 明碼
			log.Printf("進行權限:%v 操作失敗 原因:%v", PermissionsName, err)
			PermissionChangeFail += " 帳號刪除失敗 "
		} else {

			// Salt
			if err := database.MyConnect.Table("rip_current_administrator_salt").Where("userID = ?", Target.UserID).Select("accountDeletePermissionsSalt").Find(&salt).Error; err != nil {

				// 調用資料庫發生錯誤 鹽值
				log.Printf("進行權限:%v 操作失敗 原因:%v", PermissionsName, err)
				PermissionChangeFail += " 帳號刪除失敗 "
			} else {

				// Hash
				if err := database.MyConnect.Table("rip_current_administrator_Hash").Where("userID = ?", Target.UserID).Select("accountDeletePermissionsHash").Find(&hash).Error; err != nil {

					// 調用資料庫發生錯誤 哈希值
					log.Printf("進行權限:%v 操作失敗 原因:%v", PermissionsName, err)
					PermissionChangeFail += " 帳號刪除失敗 "
				} else {

					// 三種值都取回了 驗證吧
					if checkPassword, err := PasswordCheck(password, salt); err != nil {

						// 發生錯誤
						log.Printf("進行權限:%v 操作失敗 原因:%v", PermissionsName, err)
						PermissionChangeFail += " 帳號刪除失敗 "
					} else {

						if hash != checkPassword {

							// 不符合 禁止權限操作
							log.Printf("進行權限:%v 操作失敗 原因:%v", PermissionsName, errors.New("Hash並不相符不允許操作"))
							PermissionChangeFail += " 沒有權限帳號刪除失敗 "
						} else {

							// 刪除帳號吧
							log.Print("觸發了")
							if err := OTP_Create(Target); err != nil { //進行OTP驗證 在進行指令操作

								// 錯誤
								c.JSON(400, gin.H{"": ""})
								log.Printf("OTP錯誤 原因:%v", err)
							}
						}
					}
				}
			}
		}
	}

	// 成功
	if PermissionChangeFail != "" {

		c.JSON(400, gin.H{"message": fmt.Sprintf("有操作失敗:%v", PermissionChangeFail)})
	} else {
		c.JSON(200, gin.H{"message": "操作全部成功"})
	}
}

// 驗證OTP並執行操作 參考結構:3 步驟2:使用UserOTP(帳號和驗證碼) 成功就處理之前的操作
func Permissions_OTP_Check(c *gin.Context) {

	// 變數
	check := UserOTP{}
	var VerificationInformation OTP
	PermissionChangeFail := ""
	tx := database.MyConnect.Begin()

	// 綁定
	if err := c.ShouldBindJSON(&check); err != nil {

		// 錯誤
		c.JSON(400, gin.H{"message": "綁定失敗"})
		log.Printf("綁定失敗 原因:%S", err)
		return
	}

	// 取值
	if value, ok := templates.OperationCache.Load(check.UserID); ok {

		if VerificationInformation, ok = value.(OTP); !ok {

			// 錯誤
			c.JSON(400, gin.H{"message": "驗證失敗"})
			log.Print("取值失敗")
			return
		}
	}

	//
	if isVaild := totp.Validate(check.VerificationCode, VerificationInformation.VerificationCode); !isVaild {

		// 錯誤 不相同
		c.JSON(400, gin.H{"message": "驗證失敗"})
		log.Printf("管理員:%v 驗證失敗", VerificationInformation.UserID)
		return
	}

	// 停止使用者帳號

	// 停止使用者上傳圖片的權限

	// 自動更新各項權限的驗證碼

	// 進行權限認證: 刪除用戶帳號
	if VerificationInformation.other.AccountDeletePermissions == "1" {

		log.Printf("進來囉:%v", VerificationInformation.other.OperationTarget)

		// 進行操作
		if err := tx.Table("rip_current_member").Where("memberGmail = ?", VerificationInformation.other.OperationTarget).Delete(&NumberStruct{}).Error; err != nil {

			// 錯誤
			log.Printf("進行權限:%v 資料庫操作失敗 原因:%v", "accountDeletePermissions", errors.New("Hash並不相符不允許操作"))
			PermissionChangeFail += " 沒有權限帳號刪除失敗 "
			tx.Rollback()
		} else {

			// 成功
			templates.OperationCache.Delete(VerificationInformation.UserID) // 清掉權限請求紀錄
			log.Printf("清掉管理者:%v 在時間:%v 新增的權限操作請求", VerificationInformation.UserID, VerificationInformation.InsertTime.Format("2006-01-02 15:04:05"))
		}
	}

	// 結果
	if PermissionChangeFail != "" {

	} else {

		// 成功
		c.JSON(200, gin.H{"message": "操作成功"})
		log.Print("權限操作成功")
		tx.Commit()
	}
}

// 管理者權限更新 #目前無法管理那些管理者沒有哪些權限
func UpdatePermissions(mode int, NewAdministrator Permission, tx *gorm.DB) (int, error) {

	// 變數
	var All_Administrator []Permission // 紀錄全部管理者

	if tx == nil {

		tx = database.MyConnect
	}

	// 權限金鑰是定期更新 但有可能會有新增的管理員
	switch mode {
	case 1:

		// 調用資料庫取回全部使用者
		if err := tx.Table("rip_current_administrator_permissions").Find(&All_Administrator).Error; err != nil {

			// 錯誤
			return 0, err
		}
	case 2:

		// 調用資料庫單一使用者
		if err := tx.Table("rip_current_administrator_permissions").Where("userID = ?", NewAdministrator.UserID).Find(&All_Administrator).Error; err != nil {

			// 錯誤
			return 0, err
		}
	}

	// 更新全部使用者的金鑰
	for _, target := range All_Administrator {

		// 產生新的明碼 更新:suspendedPermissions
		if password, err := generateRandomPassword(10); err != nil {

			// 錯誤
			return 0, err

		} else { // 新密碼產生成功

			// 產生hash和salt
			if hash, salt, err := HashArgon2(password); err != nil {

				// 錯誤
				return 0, err

			} else { // 產生hash和salt成功

				// 上傳到資料庫 更新管理員權限_明碼 suspendedPermissions
				if err := tx.Table("rip_current_administrator_permissions").Where("userID = ?", target.UserID).Updates(map[string]interface{}{"suspendedPermissions": password}).Error; err != nil {

					// 錯誤
					return 0, err
				}

				// 上傳到資料庫 更新管理員權限_鹽值 suspendedPermissionsSalt
				if err := tx.Table("rip_current_administrator_salt").Where("userID = ?", target.UserID).Updates(map[string]interface{}{"suspendedPermissionsSalt": salt}).Error; err != nil {

					// 錯誤
					return 0, err
				}

				// 上傳到資料庫 更新管理員權限_哈希值 suspendedPermissionsHash
				if err := tx.Table("rip_current_administrator_Hash").Where("userID = ?", target.UserID).Updates(map[string]interface{}{"suspendedPermissionsHash": hash}).Error; err != nil {

					// 錯誤
					return 0, err
				}
			}
		}

		// 產生新的明碼 更新:passwordPenaltyPermissions
		if password, err := generateRandomPassword(10); err != nil {

			// 錯誤
			return 0, err

		} else { // 新密碼產生成功

			// 產生hash和salt
			if hash, salt, err := HashArgon2(password); err != nil {

				// 錯誤
			} else { // 產生hash和salt成功

				// 上傳到資料庫 更新管理員權限_明碼 suspendedPermissions
				if err := tx.Table("rip_current_administrator_permissions").Where("userID = ?", target.UserID).Updates(map[string]interface{}{"passwordPenaltyPermissions": password}).Error; err != nil {

					// 錯誤
					return 0, err
				}

				// 上傳到資料庫 更新管理員權限_鹽值 suspendedPermissionsSalt
				if err := tx.Table("rip_current_administrator_salt").Where("userID = ?", target.UserID).Updates(map[string]interface{}{"passwordPenaltyPermissionsSalt": salt}).Error; err != nil {

					// 錯誤
					return 0, err
				}

				// 上傳到資料庫 更新管理員權限_哈希值 suspendedPermissionsHash
				if err := tx.Table("rip_current_administrator_Hash").Where("userID = ?", target.UserID).Updates(map[string]interface{}{"passwordPenaltyPermissionsHash": hash}).Error; err != nil {

					// 錯誤
					return 0, err
				}
			}
		}

		// 產生新的明碼 更新:photoSuspensionPermissions
		if password, err := generateRandomPassword(10); err != nil {

			// 錯誤
			return 0, err

		} else { // 新密碼產生成功

			// 產生hash和salt
			if hash, salt, err := HashArgon2(password); err != nil {

				// 錯誤
			} else { // 產生hash和salt成功

				// 上傳到資料庫 更新管理員權限_明碼 suspendedPermissions
				if err := tx.Table("rip_current_administrator_permissions").Where("userID = ?", target.UserID).Updates(map[string]interface{}{"photoSuspensionPermissions": password}).Error; err != nil {

					// 錯誤
					return 0, err
				}

				// 上傳到資料庫 更新管理員權限_鹽值 suspendedPermissionsSalt
				if err := tx.Table("rip_current_administrator_salt").Where("userID = ?", target.UserID).Updates(map[string]interface{}{"photoSuspensionPermissionsSalt": salt}).Error; err != nil {

					// 錯誤
					return 0, err
				}

				// 上傳到資料庫 更新管理員權限_哈希值 suspendedPermissionsHash
				if err := tx.Table("rip_current_administrator_Hash").Where("userID = ?", target.UserID).Updates(map[string]interface{}{"photoSuspensionPermissionsHash": hash}).Error; err != nil {

					// 錯誤
					return 0, err
				}
			}
		}

		// 產生新的明碼 更新:accountDeletePermissions
		if password, err := generateRandomPassword(10); err != nil {

			// 錯誤
			return 0, err

		} else { // 新密碼產生成功

			// 產生hash和salt
			if hash, salt, err := HashArgon2(password); err != nil {

				// 錯誤
			} else { // 產生hash和salt成功

				// 上傳到資料庫 更新管理員權限_明碼 suspendedPermissions
				if err := tx.Table("rip_current_administrator_permissions").Where("userID = ?", target.UserID).Updates(map[string]interface{}{"accountDeletePermissions": password}).Error; err != nil {

					// 錯誤
					return 0, err
				}

				// 上傳到資料庫 更新管理員權限_鹽值 suspendedPermissionsSalt
				if err := tx.Table("rip_current_administrator_salt").Where("userID = ?", target.UserID).Updates(map[string]interface{}{"accountDeletePermissionsSalt": salt}).Error; err != nil {

					// 錯誤
					return 0, err
				}

				// 上傳到資料庫 更新管理員權限_哈希值 suspendedPermissionsHash
				if err := tx.Table("rip_current_administrator_Hash").Where("userID = ?", target.UserID).Updates(map[string]interface{}{"accountDeletePermissionsHash": hash}).Error; err != nil {

					// 錯誤
					return 0, err
				}
			}
		}

	}

	// 成功
	return len(All_Administrator), nil
}

// 觸發rip_current_administrator_permissions資料表的更新 每24小時
func SchedulePermissionUpdates() {

	// 變數
	ticker := time.NewTicker(1 * time.Hour) // 每天更新一次
	defer ticker.Stop()

	// 伺服器啟動 強制更新一次
	if count, err := UpdatePermissions(1, Permission{}, nil); err != nil {

		// 錯誤
		log.Printf("更新管理員權限失敗 原因:%v", err)
	} else {

		// 成功
		log.Printf("權限更新成功 共計:%v筆", count)
	}

	// 運行無盡迴圈
	for range ticker.C {

		// 執行權限更新的邏輯
		if count, err := UpdatePermissions(1, Permission{}, nil); err != nil {

			// 錯誤
			log.Printf("更新管理員權限失敗 原因:%v", err)
		} else {

			// 成功
			log.Printf("權限更新成功 共計:%v筆", count)
		}
	}
}

// 觸發更新每30秒一次
func ScheduleOTPUpdates() {

	for {
		time.Sleep(30 * time.Second)
		CleanOTPRecode()
	}
}

// 產生密碼
func generateRandomPassword(length int) (string, error) {

	// 變數
	const uppercase = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const lowercase = "abcdefghijklmnopqrstuvwxyz"
	const digits = "0123456789"
	const special = "$"

	allChars := uppercase + lowercase + digits + special

	password := make([]byte, length)

	// 確保至少包含一個指定的字符類型
	charSets := []string{uppercase, lowercase, digits, special}
	for i, set := range charSets {
		char, err := randCharFromSet(set)
		if err != nil {
			return "", err
		}
		password[i] = char
	}

	// 隨機填充剩餘的字符
	for i := len(charSets); i < length; i++ {
		char, err := randCharFromSet(allChars)
		if err != nil {
			return "", err
		}
		password[i] = char
	}

	// 將結果轉換為字符串
	return string(password), nil
}

// 隨機選擇字符
func randCharFromSet(charSet string) (byte, error) {

	//
	idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charSet))))
	if err != nil {
		return 0, err
	}
	return charSet[idx.Int64()], nil
}

// 建立短暫(30秒)的OTP驗證 提供管理員做驗證 #Permissions、Password、Salt
func OTP_Create(PermissionAdd Permission) error {

	// 變數
	var TOTPsecret string
	var otp string
	var operation OTP

	// 取回金鑰
	if err := database.MyConnect.Table("rip_current_administrator_permissions").Where("UserID = ?", PermissionAdd.UserID).Select("userOTPSecret").Find(&TOTPsecret).Error; err != nil {

		// 錯誤
		return fmt.Errorf("取回TOTP 失敗%v", err)

	} else {

		// 確保同時只有一筆紀錄 免得撞車
		if value, ok := templates.UserOTPCache.Load(PermissionAdd.UserID); ok {

			// 斷言
			if value, ok := value.(OTP); ok {

				// 變數 產生時間
				remain := time.Since(value.InsertTime).Seconds()
				var remaining int

				// 判斷
				// 檢查是否超過 30 秒
				if remain > 30 {
					fmt.Println("已經超過 30 秒")
				} else {
					remaining = 30 - int(remain) // 計算剩餘秒數，並取整數
					fmt.Printf("還剩下 %d 秒\n", remaining)
				}

				// 錯誤
				return fmt.Errorf("管理員:%s 已經發送請求 請:%d後在試", PermissionAdd.UserID, remaining)
			}
		}

		log.Printf("OTPsecret:%v", TOTPsecret)

		// 產生驗證碼
		if otp, err = totp.GenerateCode(TOTPsecret, time.Now()); err != nil {

			// 錯誤
			return fmt.Errorf("產生驗證碼失敗 原因:%v", err)
		}

		// 修改
		operation.UserID = PermissionAdd.UserID
		operation.VerificationCode = TOTPsecret // 金鑰
		operation.other = PermissionAdd
		operation.InsertTime = time.Now()

		// 存放
		templates.OperationCache.Store(PermissionAdd.UserID, operation)

		// 發送給客戶端
		SendGmail(operation.UserID, otp, "2")
		log.Printf("驗證碼:%v", otp)
	}

	// 回傳
	return nil
}

// 清除OTP紀錄
func CleanOTPRecode() {

	// 變數
	now := time.Now()
	var count int64

	// 開始操作
	templates.OperationCache.Range(func(key, value any) bool {

		count++

		// 取值
		if checkTime, ok := value.(OTP); !ok {

			// 錯誤
			return true

		} else {

			// 成功 來驗證
			if now.Sub(checkTime.InsertTime) > 30*time.Second {
				templates.OperationCache.Delete(checkTime.UserID)
				log.Printf("清掉管理者:%v 在時間:%v", checkTime.UserID, time.Now().Format("2006-01-02 15:04:05"))
				//log.Printf("%v", templates.OperationCache)
			}
			return true
		}
	})

	// 輸出
	if count != 0 {
		log.Printf("管理員OTP數據總量:%v", count)
	}
}

// 發送gmail #目標Gmail、驗證碼
func SendGmail(administrator string, VerificationCode string, mod string) {

	switch mod {
	case "1": // 一般使用者 密碼修改
		// 設定寄件人、密碼、SMTP 伺服器和端口
		sender := "Alderonius2002@gmail.com"
		password := "aydk vyew bhcj tkmz" // 應用程式密碼
		smtpHost := "smtp.gmail.com"
		smtpPort := 587 //587（TLS）或 465（SSL）。

		// 建立郵件訊息
		m := gomail.NewMessage()
		m.SetHeader("From", sender)
		m.SetHeader("To", administrator)
		m.SetHeader("Subject", "離岸流-'忘記密碼")
		m.SetBody("text/plain", fmt.Sprintf("用戶:%v 的忘記密碼驗證碼為:%v 請盡快輸入 有效時間為30秒!!", administrator, VerificationCode))

		// 建立 SMTP 撥號器
		d := gomail.NewDialer(smtpHost, smtpPort, sender, password)

		// 發送郵件
		if err := d.DialAndSend(m); err != nil {
			log.Fatalf("無法發送郵件: %v", err)
		}

		log.Println("郵件發送成功！")
	case "2": // 管理員 權限操作
		// 設定寄件人、密碼、SMTP 伺服器和端口
		sender := "Alderonius2002@gmail.com"
		password := "aydk vyew bhcj tkmz" // 應用程式密碼
		smtpHost := "smtp.gmail.com"
		smtpPort := 587 //587（TLS）或 465（SSL）。

		// 建立郵件訊息
		m := gomail.NewMessage()
		m.SetHeader("From", sender)
		m.SetHeader("To", administrator)
		m.SetHeader("Subject", "管理員操作驗證碼")
		m.SetBody("text/plain", fmt.Sprintf("管理員帳號:%v 的權限操作驗證碼為:%v 請盡快輸入 有效時間為30秒!!", administrator, VerificationCode))

		// 建立 SMTP 撥號器
		d := gomail.NewDialer(smtpHost, smtpPort, sender, password)

		// 發送郵件
		if err := d.DialAndSend(m); err != nil {
			log.Printf("無法發送郵件: %v", err)
		}

		log.Println("郵件發送成功！")
	}
}
