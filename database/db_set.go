package database

import (
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var MyConnect *gorm.DB
var err error

func Ms() {

	// 建立資料庫連結
	if MyConnect, err = gorm.Open(mysql.Open("root:Samyeh2002@tcp(127.0.0.1:3306)/rip_current_db?charset=utf8mb4&parseTime=True&loc=Local"), &gorm.Config{}); err != nil {
		log.Fatal(err, "\n資料庫連接失敗")
	}
	log.Print("資料庫連接成功")
}
