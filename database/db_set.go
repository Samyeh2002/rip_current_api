package database

import (
	"context"
	"log"
	"time"

	"github.com/go-redis/redis"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var Ctx = context.Background()
var Rdb *redis.Client
var MyConnect *gorm.DB
var err error

func Ms() {

	// 建立資料庫連結
	if MyConnect, err = gorm.Open(mysql.Open("root:0000@tcp(127.0.0.1:3306)/rip_current_db?charset=utf8mb4&parseTime=True&loc=Local"), &gorm.Config{}); err != nil {
		log.Fatal(err, "\n資料庫連接失敗")
	}
	log.Print("資料庫連接成功")

	// 取得*sql.DB物件
	sqlDB, _ := MyConnect.DB()

	// 設置最高連接數
	sqlDB.SetMaxOpenConns(100)

	// 設置閒置連接數
	sqlDB.SetMaxIdleConns(20)

	// 設置連接的最大存活時間
	sqlDB.SetConnMaxLifetime(45 * time.Minute)
}

// func Redis_initialization() {

// 	// 變數

// 	// 初始化
// 	Rdb = redis.NewClient(&redis.Options{
// 		Addr:     "127.0.0.1:6379",
// 		Password: "", // 沒有密碼，默認
// 		DB:       0,  // 默認 DB 0
// 	})

// 	// Ping the Redis server
// 	err := Rdb.Ping().Err()
// 	if err != nil {
// 		log.Fatalf("Could not connect to Redis: %v", err)
// 	}

// 	log.Println("Connected to Redis!")

// }
