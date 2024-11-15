package service

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg" // 加載 JPEG 格式
	_ "image/png"  // 加載 PNG 格式
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"rip_current_mod/database"
	"rip_current_mod/templates"

	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"golang.org/x/exp/rand"
)

// 簡介: 處理圖片的各種基本操作，而圖片有圖檔和資訊2種，針對這兩種進行新增、查詢、刪除、修改等操作

// 結構1-相片資訊(左邊是程式及JSON用，右邊是資料庫的，利用映射(mapping)將這兩個組合起來，避免資料庫外洩)
type InformationStruct struct {
	PhotoName           string  `gorm:"column:photoName;default:null"`           // 圖片名稱
	PhotoLocation       string  `gorm:"column:photoLocation;default:null"`       // 圖片拍攝位置
	PhotoCoordinate_lng string  `gorm:"column:photoCoordinate_lng;default:null"` // 圖片所在的經度
	PhotoCoordinate_lat string  `gorm:"column:photoCoordinate_lat;default:null"` // 圖片所在的緯度
	PhotoFilming_time   string  `gorm:"column:photoFilming_time;default:null"`   // 圖片被拍攝的時間
	PhotoPosition       string  `gorm:"column:photoPosition;default:null"`       // 圖片被拍攝時的方位
	LikeQuantity        int     `gorm:"column:likeQuantity"`                     // 圖片被按讚的數量
	LocationCode        string  `gorm:"column:locationcode;default:null"`        // 根據圖片拍攝位置轉換的區域代碼
	IsLike              bool    `gorm:"-"`                                       // 用來查看圖片是否被按讚
	PhotoHight          float64 `gorm:"-"`
	PhotoWidth          float64 `gorm:"-"`
}

type Response struct {
	Images []Image `json:"images"`
}

type Image struct {
	Results []interface{} `json:"results"`
}

// 結構2-用於功能X #
type FolderRequestStruct struct {
	FolderCodeName string // 目標資料夾的代號
	FolderSequence string // 期望的排序方式
}

// 沒黨人 要改
// 功能1-新增圖片_單張_使用POST (ver.3 改版完成 現在可以新增圖片時添加圖片資訊) #需要5個參數(PhotoLocation、PhotoCoordinate_lng、PhotoCoordinate_lat、PhotoFilming_time、PhotoPosition)
func Photo_operation_post_one(c *gin.Context) {

	// 變數
	photoInfo := InformationStruct{}
	var wg sync.WaitGroup
	var bytesCopied int64
	tx := database.MyConnect.Begin() // 紀錄資料庫狀態
	errChan := make(chan error, 1)   // 通知任務狀態
	defer close(errChan)

	// 檢查請求體
	//templates.Http_information_display(c.Request)

	// 解析請求中的表單數據
	err := c.Request.ParseMultipartForm(10 << 20) // 10MB 的上限
	if err != nil {
		log.Print("無法解析表單數據")
		c.JSON(500, gin.H{"message": "無法解析表單數據"})
		return
	}

	// 獲取表單中的檔案(類似key-value)
	file, handler, err := c.Request.FormFile("image")
	if err != nil {
		log.Print("找不到文件欄位或文件")
		c.JSON(400, gin.H{"message": "找不到文件欄位或文件"})
		return
	}
	defer file.Close()

	// 檢查文件類型
	ext, isValid := templates.Picture_file_check(file)
	if !isValid {
		c.JSON(http.StatusBadRequest, gin.H{"message": "不支持的文件类型"})
		return
	}

	// 檢查是否有副檔名 沒有就添加
	if !strings.Contains(handler.Filename, ".") {

		// 添加副檔名
		photoInfo.PhotoName = fmt.Sprintf("%s.%s", handler.Filename, ext)

	} else {

		// 按照原定計畫
		photoInfo.PhotoName = handler.Filename

	}

	// 擷取資訊
	info := c.PostForm("information")
	if err := json.Unmarshal([]byte(info), &photoInfo); err != nil {
		log.Printf("Invalid JSON: %s", err)
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid JSON"})
		return
	}

	log.Printf("測試 %v", photoInfo)
	if photoInfo.PhotoFilming_time == "" {
		log.Print("Hello")
	}
	InformationStructCheck(&photoInfo)
	log.Printf("測試2 %v", photoInfo)

	// 新增到資料庫
	if err := tx.Table("rip_current_information").Create(&photoInfo).Error; err != nil {
		log.Print("新增到資料庫失敗")
		c.JSON(200, gin.H{"message": "新增圖片資料到資料庫失敗"})
		tx.Rollback() // 回滾事務
		return
	}

	// 計數
	wg.Add(1)

	// 創建圖片
	go func() {

		// 額外任務
		defer wg.Done()

		// 創建檔案來接收要複製的檔案
		dst, err := os.Create(filepath.Join(templates.GlobalRootPath, photoInfo.PhotoName))
		if err != nil {
			log.Printf("無法創建目標文件 原因:%v", err)
			errChan <- err
			return
		}
		defer dst.Close()

		// 複製目標到創建的檔案
		bytesCopied, err = io.Copy(dst, file)
		if err != nil {
			log.Print("無法複製文件內容")
			os.Remove(dst.Name()) // 刪除空檔案
			errChan <- err
			return
		}

		// 檢查是否複製了任何內容
		if bytesCopied == 0 {
			log.Print("複製的內容為空")
			os.Remove(dst.Name()) // 刪除空檔案
			errChan <- err
			return
		}

		//
		file.Seek(0, 0)
		img, _, err := image.DecodeConfig(file)
		if err != nil {
			c.JSON(500, gin.H{"message": fmt.Sprintf("無法讀取圖片信息: %v", err)})
			return
		}
		log.Printf("%v %v", img.Height, img.Width)

		// 提交事務
		errChan <- nil // 成功標誌

	}()

	wg.Wait()

	// 檢查 goroutine 執行結果
	if err := <-errChan; err != nil {
		c.JSON(500, gin.H{"message": "圖片上傳失敗"})
		if err := os.RemoveAll(filepath.Join(templates.GlobalRootPath, photoInfo.PhotoName)); err != nil {

			// 錯誤
			log.Printf("刪除:%v失敗 原因:%v", filepath.Join(templates.GlobalRootPath, photoInfo.PhotoName), err)
		}
		tx.Rollback()
		return
	}

	// 回傳
	c.JSON(200, gin.H{"message": "文件上傳成功"})
	tx.Commit()
}

// 功能2-獲取圖片_單張_使用GET (ver.1 完成 ) #提供圖片名稱(要加.png) 來獲得圖片圖檔
func Photo_operation_get_one(c *gin.Context) {

	// 變數
	filename := c.Param("filename")

	// 構建圖片文件的絕對路徑
	imagePath := filepath.Join(templates.GlobalRootPath, filename)

	// 打開圖片文件
	file, err := os.Open(imagePath)
	if err != nil {
		c.JSON(404, gin.H{"message": fmt.Sprintf("圖片 %s 不存在", imagePath)})
		return
	}
	defer file.Close()

	//
	file.Seek(0, 0)
	img, _, err := image.DecodeConfig(file)
	if err != nil {
		c.JSON(500, gin.H{"message": fmt.Sprintf("無法讀取圖片信息: %v", err)})
		return
	}
	log.Printf("%v %v", img.Height, img.Width)

	// 設置 Content-Type 頭部
	c.Header("Content-Type", "image/jpeg") // 這裡假設圖片是 JPEG 格式，根據實際情況修改

	// 重置文件指針
	file.Seek(0, 0)

	// 將圖片複製到響應主體中
	io.Copy(c.Writer, file)
}

// 功能3-獲取圖片資訊_單筆_使用GET (ver.1 完成 ) #輸入圖片名稱(要加.png) 來獲得圖片資訊
func Photo_operation_get_information(c *gin.Context) {

	// 變數
	fileName := c.Param("filename")
	jsonStruct := InformationStruct{}

	// 向資料庫調用資料
	if err := database.MyConnect.Table("rip_current_information").Where("photoName = ?", fileName).First(&jsonStruct).Error; err != nil {

		// 發生錯誤
		log.Printf("檔案:%s 項資料庫調用資料失敗", fileName)
		c.JSON(200, gin.H{"message": fmt.Sprintf("檔案:%s 項資料庫調用資料失敗", fileName)})
	}

	// 成功
	c.JSON(200, jsonStruct)
}

// 功能4 #提供目標名稱 取得該資料夾下的所有圖片資訊
func Photo_operation_get_folder_information(c *gin.Context) {

	// 變數
	var isLogin struct {
		UserGmail     string
		Sequence      int
		UserLatitude  float64 // 緯度
		UserLongitude float64 // 經度
	}
	var users []InformationStruct
	var count int64

	// 自定義時間格式的布局
	const customTimeLayout = "20060102150405"

	// 接收參數
	if err := c.ShouldBindJSON(&isLogin); err != nil {
		log.Printf("綁定失敗 原因:%s", err.Error())
		c.JSON(http.StatusBadGateway, gin.H{"message": "綁定失敗"})
		return
	}

	// 判斷用戶是否登入
	if isLogin.UserGmail != "" {

		// 驗證帳號是否存在
		if err := database.MyConnect.Table("rip_current_member").Where("memberGmail = ?", isLogin.UserGmail).Count(&count).Error; err != nil {

			// 帳號不存在
			log.Printf("登入帳號錯誤 原因:%s", err)

		} else {

			// 帳號已登入
			log.Printf("用戶已登入 名稱:%s", isLogin.UserGmail)
		}

	} else {
		log.Print("用戶未登入 請求資料")
	}

	// 连接数据库并查询所有用户记录
	if err := database.MyConnect.Table("rip_current_information").Find(&users).Error; err != nil {
		log.Printf("資料庫查詢失敗")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "資料庫查詢失敗"})
		return
	}

	// 如果用戶已登入，檢查其點贊記錄
	if isLogin.UserGmail != "" {
		var likes []struct {
			PhotoName string
		}
		// 查詢用戶按讚記錄
		if err := database.MyConnect.Table("rip_current_photo_other").Where("user_id = ?", isLogin.UserGmail).Select("photo_id as PhotoName").Find(&likes).Error; err != nil {
			log.Printf("無法查詢用戶點贊記錄: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "無法查詢用戶點贊記錄"})
			return
		}

		log.Print(likes)

		// 建立一個集合來快速檢查使用者是否按讚
		likedPhotos := make(map[string]bool)
		for _, like := range likes {
			likedPhotos[like.PhotoName] = true
		}

		// 遍歷所有照片，標記是否已被該用戶按讚
		for i := range users {
			if likedPhotos[users[i].PhotoName] {
				users[i].IsLike = true
			}
		}
	}

	//log.Print(users)

	// // 按照不同條件排序 (愛心排序:完成)
	switch isLogin.Sequence {
	case 1:
		// 按時間排序 #由新到舊
		log.Print("觸發時間排序")
		slices.SortFunc(users, func(a, b InformationStruct) int {
			t1, err1 := time.Parse(customTimeLayout, a.PhotoFilming_time)
			t2, err2 := time.Parse(customTimeLayout, b.PhotoFilming_time) // time.RFC3339 也許之後會用到
			if err1 != nil || err2 != nil {
				// 如果解析失敗，我們假設相等
				return 0
			}
			if t1.Before(t2) {
				return 1
			} else if t1.After(t2) {
				return -1
			} else {
				return 0
			}
		})

		//
		for _, i := range users {
			t, err := time.Parse("20060102150405", i.PhotoFilming_time)
			if err != nil {
				fmt.Println("解析時間字串時出錯:", err)
				return
			}

			formatted := t.Format("2006/01/02 號 15:04:05")
			fmt.Println(formatted)
		}

	case 2:
		// 按照愛心
		log.Print("觸發愛心排序")
		slices.SortFunc(users, func(a, b InformationStruct) int {
			if a.LikeQuantity > b.LikeQuantity {
				return -1
			} else if a.LikeQuantity < b.LikeQuantity {
				return 1
			} else {
				return 0
			}
		})

	case 3:
		// 默認距離排序
		log.Print("觸發距離排序")
		log.Printf("Lat:%v Lon:%v", isLogin.UserLatitude, isLogin.UserLongitude)
		log.Print(isLogin)
		for _, i := range users {
			log.Printf("%s的距離為:%.2f公尺", i.PhotoName, templates.Haversine(isLogin.UserLatitude, isLogin.UserLongitude, templates.StringChangeFloat64(i.PhotoCoordinate_lat), templates.StringChangeFloat64(i.PhotoCoordinate_lng))*1000)
		}
		slices.SortFunc(users, func(a, b InformationStruct) int {

			// 計算距離
			dist1 := templates.Haversine(isLogin.UserLatitude, isLogin.UserLongitude, templates.StringChangeFloat64(a.PhotoCoordinate_lat), templates.StringChangeFloat64(a.PhotoCoordinate_lng)) // 緯度 經度
			dist2 := templates.Haversine(isLogin.UserLatitude, isLogin.UserLongitude, templates.StringChangeFloat64(b.PhotoCoordinate_lat), templates.StringChangeFloat64(b.PhotoCoordinate_lng)) // 緯度 經度

			//
			//log.Printf("a:%s到緯度:%f經度:%f的距離為:%f", a.PhotoName, isLogin.UserLatitude, isLogin.UserLongitude, dist1)
			//log.Printf("b:%s到緯度:%f經度:%f的距離為:%f", b.PhotoName, isLogin.UserLatitude, isLogin.UserLongitude, dist2)

			//
			if dist1 > dist2 {
				//log.Print(users)
				return 1
			} else if dist1 < dist2 {
				//log.Print(users)
				return -1
			} else {
				//log.Print("啥都沒發生", "\n")
				return 0
			}

			//

		})

		// 時間排序
		slices.SortFunc(users, func(a, b InformationStruct) int {
			t1, err1 := time.Parse(customTimeLayout, a.PhotoFilming_time)
			t2, err2 := time.Parse(customTimeLayout, b.PhotoFilming_time) // time.RFC3339 也許之後會用到
			if err1 != nil || err2 != nil {
				// 如果解析失敗，我們假設相等
				return 0
			}
			if t1.Before(t2) {
				return 1
			} else if t1.After(t2) {
				return -1
			} else {
				return 0
			}
		})

	default:
		// 默認為權重排序 先距離 再時間
		slices.SortFunc(users, func(a, b InformationStruct) int {

			// 計算距離
			dist1 := templates.Haversine(isLogin.UserLatitude, isLogin.UserLongitude, templates.StringChangeFloat64(a.PhotoCoordinate_lat), templates.StringChangeFloat64(a.PhotoCoordinate_lng)) // 緯度 經度
			dist2 := templates.Haversine(isLogin.UserLatitude, isLogin.UserLongitude, templates.StringChangeFloat64(b.PhotoCoordinate_lat), templates.StringChangeFloat64(b.PhotoCoordinate_lng)) // 緯度 經度

			//
			//log.Printf("a:%s到緯度:%f經度:%f的距離為:%f", a.PhotoName, isLogin.UserLatitude, isLogin.UserLongitude, dist1)
			//log.Printf("b:%s到緯度:%f經度:%f的距離為:%f", b.PhotoName, isLogin.UserLatitude, isLogin.UserLongitude, dist2)

			//
			if dist1 > dist2 {
				//log.Print(users)
				return 1
			} else if dist1 < dist2 {
				//log.Print(users)
				return -1
			} else {
				//log.Print("啥都沒發生", "\n")
				return 0
			}

			//

		})
	}
	for _, i := range users {
		fmt.Printf("測試:%v\n", i)
	}

	// 将查询结果转换为 JSON 格式
	jsonData, err := json.Marshal(users)
	if err != nil {
		log.Printf("JSON編碼失敗")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "JSON編碼失敗"})
		return
	}

	// 返回 JSON 响应
	//log.Print(string(jsonData))
	c.Data(http.StatusOK, "application/json", jsonData)
}

// 功能5-獲取圖片_單張_使用GET_包含圖片資訊 (ver.1 完成 )
func Photo_operation_get_one_include_information(c *gin.Context) {
	// 變數
	filename := c.Param("filename")

	// 構建圖片文件的絕對路徑
	imagePath := filepath.Join(templates.GlobalRootPath, filename)

	// 打開圖片文件
	file, err := os.Open(imagePath)
	if err != nil {
		c.JSON(404, gin.H{"message": fmt.Sprintf("圖片 %s 不存在", imagePath)})
		return
	}
	defer file.Close() // 确保在函数结束时关闭文件

	// 讀取圖片文件內容
	imageBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(500, gin.H{"message": "無法讀取圖片內容"})
		return
	}

	// 從數據庫中獲取 JSON 數據
	var photoInfo InformationStruct
	if err := database.MyConnect.Table("rip_current_information").Where("photoName = ?", filename).First(&photoInfo).Error; err != nil {
		c.JSON(500, gin.H{"message": "無法從數據庫獲取圖片信息"})
		return
	}
	jsonData, err := json.Marshal(photoInfo)
	if err != nil {
		c.JSON(500, gin.H{"message": "無法編碼 JSON 數據"})
		return
	}

	// 設置 Content-Type 頭部為 multipart/mixed
	c.Header("Content-Type", "multipart/mixed")

	// 创建 Multipart Writer
	writer := multipart.NewWriter(c.Writer)
	defer writer.Close() // 确保在响应结束时关闭 Writer

	// 寫入 JSON 部分
	jsonPartHeader := make(textproto.MIMEHeader)
	jsonPartHeader.Set("Content-Type", "application/json")
	jsonPartHeader.Set("Content-Disposition", `form-data; name="information"`)
	jsonPart, err := writer.CreatePart(jsonPartHeader)
	if err != nil {
		c.JSON(500, gin.H{"message": "無法創建 JSON 部分"})
		return
	}
	if _, err := jsonPart.Write(jsonData); err != nil {
		c.JSON(500, gin.H{"message": "無法寫入 JSON 數據"})
		return
	}

	// 寫入圖片部分
	imagePartHeader := make(textproto.MIMEHeader)
	imagePartHeader.Set("Content-Type", "image/png") // 根据实际情况修改 Content-Type
	imagePartHeader.Set("Content-Disposition", `form-data; name="image"; filename="`+filename+`"`)
	imagePart, err := writer.CreatePart(imagePartHeader)
	if err != nil {
		c.JSON(500, gin.H{"message": "無法創建圖片部分"})
		return
	}
	if _, err := imagePart.Write(imageBytes); err != nil {
		c.JSON(500, gin.H{"message": "無法寫入圖片數據"})
		return
	}
}

// 功能6(最終要取代功能5)-根據資料夾名稱、排序等欄位，取得指定資料夾的所有圖片及圖檔資訊(也可以僅拿其中一個)
func Photo_operation_complete_folder(c *gin.Context) {

	// 預期方式: 根據目標

	// 變數
	folder_select_mod := c.DefaultQuery("fsm", "null")                     // 資料回傳目標 1:僅要圖片圖檔 2:僅要圖片資訊 3:全都要 預設為:2
	folder_code_name := c.DefaultQuery("fcn", "null")                      // 資料夾代碼 null就是全部資料夾 預設為:null
	folder_sequence := c.DefaultQuery("fs", "1")                           // 排序方式 1:時間排序 2:點讚排序 3:距離排序 預設為:1
	userLongitude, _ := strconv.ParseFloat(c.DefaultQuery("lon", "0"), 64) // 經度
	userLatitude, _ := strconv.ParseFloat(c.DefaultQuery("lat", "0"), 64)  // 緯度

	// 結構檢查
	//templates.Http_information_display(c.Request)

	// 判斷資料需求
	switch folder_select_mod {
	case "1": // 僅要圖片圖檔

		switch folder_code_name {
		case "null": // 歷遍資料夾下的所有圖片
			fmt.Println("進來了")

			// // 目標資料夾
			folderPath := templates.GlobalRootPath

			// 歷遍資料夾並回傳
			templates.FolderTraverseAndReturn(folderPath, c)

		case "海灘A":

			// 變數
			locationName := "海灘A"
			locationPath := filepath.Join(templates.GlobalRootPath, locationName)

			// 歷遍資料夾並回傳
			templates.FolderTraverseAndReturn(locationPath, c)

		default:
			//
			fmt.Print("沒有給參數fcn")
		}
	case "2": // 僅要圖片資訊

		switch folder_code_name {
		case "null": // 歷便資料庫將所有圖片的圖片資訊取出

			// 變數
			var target []InformationStruct
			selectTarget := folder_code_name
			limit, _ := strconv.Atoi(c.DefaultQuery("limit", "0"))   // 假設最大的資料取回數
			offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0")) // 假設要跳過幾筆資料

			// 取回資料
			if DataBaseDataReturn(&target, selectTarget, limit, offset) != nil {

				// 回傳錯誤
				c.JSON(500, gin.H{"message": "從資料庫調用資料失敗"})
				log.Println("從資料庫調用資料失敗")
				return
			}

			// 進行條件排序
			if err := DataSequence(&target, folder_sequence, userLongitude, userLatitude); err != nil {

				// 發生錯誤
				c.JSON(400, gin.H{"錯誤!!!": fmt.Sprintf("經緯度皆為:0 用戶傳送的經度:%v 用戶傳送的緯度:%v", userLongitude, userLatitude)})
				log.Print(err.Error())
				return
			}

			// 成功 進行回傳
			c.JSON(200, target)

		case "":
		}
	case "3": // 同時要求

		// 簡介: 判斷是否有指定要哪一區的，否則全部都丟回去，圖片和圖片資訊只要有一關出問題就算失敗

		// 變數
		PhotoPath := templates.GetAreaPath(templates.GlobalRootPath, folder_code_name)
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "0"))   // 假設最大的資料取回數
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0")) // 假設要跳過幾筆資料
		var photoInformation []InformationStruct

		// 設置 Content-Type 為 application/zip 以回傳 ZIP 文件
		c.Header("Content-Type", "application/zip")
		c.Header("Content-Disposition", "attachment; filename=images_with_metadata.zip")

		// 先理圖片資訊
		DataBaseDataReturn(&photoInformation, folder_code_name, limit, offset) // 圖片取回
		if err := DataSequence(&photoInformation, folder_sequence, userLongitude, userLatitude); err != nil {

			// 發生錯誤
			c.JSON(400, gin.H{"錯誤!!!": fmt.Sprintf("經緯度皆為:0 用戶傳送的經度:%v 用戶傳送的緯度:%v", userLongitude, userLatitude)})
			log.Print(err.Error())
			return
		} // 圖片資訊排序

		// 創建 ZIP 壓縮器
		zipWriter := zip.NewWriter(c.Writer)
		defer zipWriter.Close()

		// 創建 JSON 文件部分並添加到 ZIP 文件
		jsonFile, err := zipWriter.Create("metadata.json")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create JSON in ZIP: %v", err)})
			return
		}
		jsonBytes, err := json.MarshalIndent(photoInformation, "", "	")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to marshal JSON: %v", err)})
			return
		}
		_, err = jsonFile.Write(jsonBytes)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to write JSON to ZIP: %v", err)})
			return
		}

		// // 後處理圖片打包，遍歷資料夾，將圖片加入 ZIP 檔案
		err = filepath.Walk(PhotoPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// 檢查是不是文件，並且過濾出圖片文件 (根據擴展名)
			if !info.IsDir() && (strings.HasSuffix(info.Name(), ".jpg") || strings.HasSuffix(info.Name(), ".png")) {
				// 打開文件
				file, err := os.Open(path)
				if err != nil {
					return err
				}
				defer file.Close()

				// 創建 ZIP 文件中的對應條目
				zipFile, err := zipWriter.Create(info.Name())
				if err != nil {
					return err
				}

				// 將文件內容寫入 ZIP 條目中
				_, err = io.Copy(zipFile, file)
				if err != nil {
					return err
				}
			}

			return nil
		})

		// 如果出錯，返回錯誤信息
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create ZIP file: %v", err))
			return
		}
	default:

		// 簡介: 使用者沒有給資料回傳目標(fsm)，但可能有給挑選條件(fcn)和排序方式(fs)，預設為全部回傳

		// 變數
		var target []InformationStruct
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "0"))   // 假設最大的資料取回數
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0")) // 假設要跳過幾筆資料

		// 取回資料
		if DataBaseDataReturn(&target, folder_code_name, limit, offset) != nil {

			// 回傳錯誤
			c.JSON(500, gin.H{"message": "從資料庫調用資料失敗"})
			log.Println("從資料庫調用資料失敗")
			return
		}

		// 進行條件排序 #沒給參數就是時間排序
		if err := DataSequence(&target, folder_sequence, userLongitude, userLatitude); err != nil {

			// 發生錯誤
			c.JSON(400, gin.H{"錯誤!!!": fmt.Sprintf("經緯度皆為:0 用戶傳送的經度:%v 用戶傳送的緯度:%v", userLongitude, userLatitude)})
			log.Print(err.Error())
			return
		}

		// 成功 進行回傳
		c.JSON(200, target)
	}
}

// 功能7: 劃出離岸流的圖片並回傳
func Photo_operation_painting_rip(c *gin.Context) {

	// 變數
	var fileName string

	//templates.Http_information_display(c.Request)

	// 解析請求中的表單數據
	err := c.Request.ParseMultipartForm(10 << 20) // 10MB 的上限
	if err != nil {
		log.Print("無法解析表單數據")
		c.JSON(500, gin.H{"message": "無法解析表單數據"})
		return
	}

	// 獲取表單中的檔案(類似key-value)
	file, handler, err := c.Request.FormFile("image")
	if err != nil {
		log.Print("找不到文件欄位或文件")
		c.JSON(400, gin.H{"message": "找不到文件欄位或文件"})
		return
	}
	defer file.Close()

	// 檢查文件類型
	ext, isValid := templates.Picture_file_check(file)
	if !isValid {
		c.JSON(http.StatusBadRequest, gin.H{"message": "不支持的文件类型"})
		return
	}

	// 檢查是否有副檔名 沒有就添加
	if !strings.Contains(handler.Filename, ".") {

		// 添加副檔名
		parts := strings.Split(handler.Filename, ".")
		name := containsOnlyLettersAndDigits(parts[0])
		fileName = fmt.Sprintf("%s.%s", name, ext)
	} else {

		// 按照原定計畫
		parts := strings.Split(handler.Filename, ".")
		name := containsOnlyLettersAndDigits(parts[0])
		fileName = fmt.Sprintf("%s.%s", name, parts[len(parts)-1])
	}

	log.Printf("名稱:%v", fileName)

	// 創建檔案來接收要複製的檔案
	dst, err := os.Create(filepath.Join(templates.GlobalRootPath, fileName))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "失敗"})
		log.Printf("無法創建目標文件 原因:%v", err)
		return
	}
	defer dst.Close()

	// 複製目標到創建的檔案
	bytesCopied, err := io.Copy(dst, file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "失敗"})
		log.Print("無法複製文件內容")
		os.Remove(dst.Name()) // 刪除空檔案
		return
	}

	// 檢查是否複製了任何內容
	if bytesCopied == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "失敗"})
		log.Print("複製的內容為空")
		os.Remove(dst.Name()) // 刪除空檔案
		return
	}

	// 調用python--
	cmd := exec.Command("C:/Users/PC3/AppData/Local/Programs/Python/Python313/python.exe", "C:\\Users\\PC3\\Documents\\Python\\test3.py", filepath.Join(templates.GlobalRootPath, fileName))

	// 執行並獲取輸出-- filepath.Join(templates.GlobalRootPath, photoInfo.PhotoName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "失敗"})
		log.Println("執行 Python 腳本時出錯1:", err)
		str := string(output)
		log.Println("執行 Python 腳本時出錯2:", str)
		return
	}

	// --
	log.Println("Python 輸出:", string(output))

	//
	jsonPart := strings.Split(string(output), "ResultError ")[0]

	//
	var response Response
	err = json.Unmarshal([]byte(jsonPart), &response)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}

	// 檢查 results 是否為空
	if len(response.Images) > 0 && len(response.Images[0].Results) == 0 {

		log.Println("Results is empty.")
		c.JSON(400, gin.H{"message": "錯誤"})
		return
	} else {
		log.Println("Results contains data.")
	}

	// 打開生成的圖片文件以進行覆蓋
	generatedImage, err := os.Open(filepath.Join(templates.GlobalPythonPhotoPath, fileName))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "失敗"})
		log.Print("無法打開生成的圖片文件")
		return
	}
	defer generatedImage.Close()

	// 使用新的圖片內容覆蓋原來的 dst
	dst, err = os.Create(filepath.Join(templates.GlobalRootPath, fileName)) // 重新創建檔案以進行覆蓋
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "失敗"})
		log.Print("無法創建目標文件以覆蓋")
		return
	}
	defer dst.Close()

	// 將生成的圖片內容寫入到 dst
	bytesCopied, err = io.Copy(dst, generatedImage)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "失敗"})
		log.Print("無法複製生成的圖片內容")
		os.Remove(dst.Name()) // 刪除空檔案
		return
	}

	// 檢查是否複製了任何內容
	if bytesCopied == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "失敗"})
		log.Print("複製的內容為空")
		os.Remove(dst.Name()) // 刪除空檔案
		return
	}

	// 構建圖片文件的絕對路徑
	imagePath := filepath.Join(templates.GlobalRootPath, fileName)

	// 打開圖片文件
	file, err = os.Open(imagePath)
	if err != nil {
		c.JSON(404, gin.H{"message": fmt.Sprintf("圖片 %s 不存在", imagePath)})
		return
	}
	defer file.Close()

	//
	file.Seek(0, 0)
	img, _, err := image.DecodeConfig(file)
	if err != nil {
		c.JSON(500, gin.H{"message": fmt.Sprintf("無法讀取圖片信息: %v", err)})
		return
	}
	log.Printf("%v %v", img.Height, img.Width)

	// 設置 Content-Type 頭部
	c.Header("Content-Type", "image/JPEG") // 這裡假設圖片是 JPEG 格式，根據實際情況修改

	// 重置文件指針
	file.Seek(0, 0)

	// 將圖片複製到響應主體中
	io.Copy(c.Writer, file)
}

//通用方法----------------------------------------------------------------------------------------------------------

// 根據區域碼從資料庫取回值 接收3個參數(區域編號、取回數量、跳過數量)
func DataBaseDataReturn(Target *[]InformationStruct, AreaCode string, Limit int, Offset int) error {

	// 簡介:設有3種參數，重要的是搜尋參數，為null的話就全部回傳，反之就根據區域欄位判斷

	// 變數
	dataBase := database.MyConnect.Table("rip_current_information")

	// 進行判斷
	if AreaCode != "null" {

		// 有指定搜尋目標
		dataBase = dataBase.Where("locationcode = ?", AreaCode)
	}

	if Limit != 0 {

		// 有指定取回數量
		dataBase = dataBase.Limit(Limit)
	}

	if Offset != 0 {

		// 有指定跳過數量
		dataBase = dataBase.Offset(Offset)
	}

	// 進行查詢
	if err := dataBase.Find(&Target).Error; err != nil {
		// 錯誤處理
		return err
	}

	return nil
}

// 對資料進行排序 1:時間 2:點讚 3:距離 預設為時間排序 #如果選擇距離排序但經緯度皆為0 則以錯誤回傳
func DataSequence(Target *[]InformationStruct, Sequence string, Userlongitude float64, Userlatitude float64) error {

	// 簡介: 藉由使用者傳送過來的經緯度

	// 變數
	const customTimeLayout = "20060102150405" // 自定義時間格式的布局

	// 判斷排序條件
	switch Sequence {
	case "1": // 時間排序 #由新到舊

		// 進入提示
		log.Print("觸發時間排序")

		// 排序
		slices.SortFunc(*Target, func(a, b InformationStruct) int {

			// 變數
			t1, err1 := time.Parse(customTimeLayout, a.PhotoFilming_time)
			t2, err2 := time.Parse(customTimeLayout, b.PhotoFilming_time) // time.RFC3339 也許之後會用到
			if err1 != nil || err2 != nil {
				// 如果解析失敗，我們假設相等
				return 0
			}
			if t1.Before(t2) {
				return 1
			} else if t1.After(t2) {
				return -1
			} else {
				return 0
			}
		})

		// 正常
		return nil

	case "2": // 按讚排序 #由多到少

		// 進入提示
		log.Print("觸發愛心排序")

		// 排序
		slices.SortFunc(*Target, func(a, b InformationStruct) int {

			// 排序比較條件
			if a.LikeQuantity > b.LikeQuantity {
				return -1
			} else if a.LikeQuantity < b.LikeQuantity {
				return 1
			} else {
				return 0
			}
		})

		// 正常
		return nil

	case "3": // 距離排序 #由近到遠

		// 進入提示
		log.Print("觸發距離排序")

		// 確認經緯度不為0
		if Userlongitude == 0 && Userlatitude == 0 {
			return fmt.Errorf("經緯度皆為:0 經度:%v 緯度:%v", Userlongitude, Userlatitude)
		}

		// 排序
		slices.SortFunc(*Target, func(a, b InformationStruct) int {

			// 計算距離
			dist1 := templates.Haversine(Userlatitude, Userlongitude, templates.StringChangeFloat64(a.PhotoCoordinate_lat), templates.StringChangeFloat64(a.PhotoCoordinate_lng)) // 緯度 經度
			dist2 := templates.Haversine(Userlatitude, Userlongitude, templates.StringChangeFloat64(b.PhotoCoordinate_lat), templates.StringChangeFloat64(b.PhotoCoordinate_lng)) // 緯度 經度

			// 排序比較
			if dist1 > dist2 {

				return 1
			} else if dist1 < dist2 {

				return -1
			} else {

				return 0
			}
		})

		// 正常
		return nil

	default:
		fmt.Println("DataBaseDataReturn的default被觸發 不應該")
		return nil
	}
}

// 檢查結構 還可以幫忙補充
func InformationStructCheck(Target *InformationStruct) {

	// 變數
	const customTimeLayout = "20060102150405"
	currentTime := time.Now()

	// 時間檢查
	if Target.PhotoFilming_time == "" {

		// 自動補充時間
		Target.PhotoFilming_time = currentTime.Format(customTimeLayout)
	}
}

// 正規表達式
func containsOnlyLettersAndDigits(input string) string {
	// 定義正則表達式，匹配非字母和數字的字符

	// 變數
	re := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, 6)

	if re.MatchString(input) {

		//　符合
		return input

	} else {

		// 不符合
		rand.Seed(uint64(time.Now().UnixNano())) // 設定隨機數種子
		for i := range result {
			result[i] = letters[rand.Intn(len(letters))] // 隨機選擇一個字符
		}

		// 回傳
		return string(result)

	}
}

// 網頁專用--------------------------------------------------------------------------------------------------------------------------------------

// 和功能6相關 用來處理網頁圖片的問題 #參數mod=1(網頁載入用) or 2(完整內容)
func Web_Photo_classification(c *gin.Context) {

	// 變數
	mod := c.DefaultQuery("mod", "1")

	// 開始
	switch mod {
	case "1": // 網頁載入

		// 打開 ZIP 文件
		zipFile, err := os.Open(templates.WebLoadPath)
		if err != nil {
			c.String(http.StatusInternalServerError, "無法打開 ZIP 文件: %v", err)
			return
		}
		defer zipFile.Close()

		// 設定回應頭，告知瀏覽器這是個 ZIP 文件
		c.Header("Content-Disposition", "attachment; filename=photos.zip")
		c.Header("Content-Type", "application/zip")

		// 使用 Stream 將 ZIP 文件串流到客戶端
		_, err = io.Copy(c.Writer, zipFile)
		if err != nil {
			c.String(http.StatusInternalServerError, "無法傳輸 ZIP 文件: %v", err)
			return
		}

		// 成功

	case "2": // 完整內容

		// 打開 ZIP 文件
		zipFile, err := os.Open(templates.WebCompletePath)
		if err != nil {
			c.String(http.StatusInternalServerError, "無法打開 ZIP 文件: %v", err)
			return
		}
		defer zipFile.Close()

		// 設定回應頭，告知瀏覽器這是個 ZIP 文件
		c.Header("Content-Disposition", "attachment; filename=photos.zip")
		c.Header("Content-Type", "application/zip")

		// 使用 Stream 將 ZIP 文件串流到客戶端
		_, err = io.Copy(c.Writer, zipFile)
		if err != nil {
			c.String(http.StatusInternalServerError, "無法傳輸 ZIP 文件: %v", err)
			return
		}
	}
}

// 監聽器 更新緩存圖片zip
func Web_Schedule_photo() {

	// 初始化 fsnotify.Watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// 要監聽的資料夾路徑
	directoryToWatch := templates.GlobalPythonPhotoPath

	// 開始監聽資料夾
	err = watcher.Add(directoryToWatch)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("開始監聽資料夾變化...")

	go func() {
		for {

			//
			select {

			//
			case event, ok := <-watcher.Events:

				//
				if !ok {
					return
				}

				//
				log.Println("event:", event)

				//
				if event.Has(fsnotify.Write) {
					log.Println("modified file:", event.Name)
				}

				//
				if err := Web_update_load_photo(); err != nil {
					log.Printf("發生錯誤 原因:%v", err)
				}

				//
				if err := Web_update_complete_photo(); err != nil {
					log.Printf("發生錯誤 原因:%v", err)
				}

			//
			case err, ok := <-watcher.Errors:
				//
				if !ok {
					return
				}
				log.Println("監聽錯誤:", err)
			}
		}
	}()

	// // 監聽事件
	// for {
	// 	select {
	// 	case event := <-watcher.Events: //

	// 		//
	// 		log.Printf("發生變化: %s\n", event.Name)

	// 		//

	// 	case err := <-watcher.Errors: //
	// 		if err != nil {

	// 		}
	// 	}

	// 	log.Print("su3cl3")
	// }

}

// 更新載入圖片
func Web_update_load_photo() error {

	// 變數
	photo_path := templates.WebLoadPath             // 資料夾路徑
	photo_quantity := templates.WebLoadPathQuantity // 資料夾應該要包含的圖片數量
	result := []InformationStruct{}

	// 調用資料庫 取回最新的N筆資料 查詢小於等於當前時間的資料 根據時間差排序 限制查詢結果為 10 筆資料
	if err := database.MyConnect.Table("rip_current_information").Order("photoFilming_datetime DESC").Limit(photo_quantity).Find(&result).Error; err != nil {

		// 錯誤
		return fmt.Errorf("發生錯誤 原因:%v", err)
	}

	// 處理zip檔案
	if err := os.Remove(photo_path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("無法刪除舊的 ZIP 檔案: %v", err)
	}

	// 建立 ZIP 文件
	zipFile, err := os.Create(photo_path)
	if err != nil {
		return fmt.Errorf("cannot create zip file: %v", err)
	}
	defer zipFile.Close()

	// 創建 zip.Writer 寫入 ZIP 文件
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// 創建 JSON 文件部分並添加到 ZIP 文件
	jsonFile, err := zipWriter.Create("metadata.json")
	if err != nil {
		return err
	}

	//
	jsonBytes, err := json.MarshalIndent(result, "", "	")
	if err != nil {
		return err
	}

	//
	_, err = jsonFile.Write(jsonBytes)
	if err != nil {
		return err
	}

	// 遍歷查詢結果中的圖片名稱，並添加到 ZIP 檔案
	for _, image := range result {

		// 路徑
		imagePath := filepath.Join(templates.GlobalRootPath, image.PhotoName)
		//log.Printf("名稱:%s", imagePath)

		// 開啟圖片檔案
		imageFile, err := os.Open(imagePath)
		if err != nil {
			return fmt.Errorf("無法開啟圖片檔案 %s: %v", image.PhotoName, err)
		}
		defer imageFile.Close()

		// 創建 ZIP 檔案內部條目
		zipEntryWriter, err := zipWriter.Create(image.PhotoName)
		if err != nil {
			return fmt.Errorf("無法創建 ZIP 條目: %v", err)
		}

		// 複製圖片內容到 ZIP
		_, err = io.Copy(zipEntryWriter, imageFile)
		if err != nil {
			return fmt.Errorf("無法寫入圖片檔案到 ZIP: %v", err)
		}
	}

	//
	return nil
}

// 更新完整圖片
func Web_update_complete_photo() error {
	// 變數
	photo_path := templates.WebCompletePath // 資料夾路徑
	result := []InformationStruct{}

	// 調用資料庫 取回最新的N筆資料 查詢小於等於當前時間的資料 根據時間差排序 限制查詢結果為 10 筆資料
	if err := database.MyConnect.Table("rip_current_information").Order("photoFilming_datetime DESC").Find(&result).Error; err != nil {

		// 錯誤
		return fmt.Errorf("發生錯誤 原因:%v", err)
	}

	// 處理zip檔案
	if err := os.Remove(photo_path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("無法刪除舊的 ZIP 檔案: %v", err)
	}

	// 建立 ZIP 文件
	zipFile, err := os.Create(photo_path)
	if err != nil {
		return fmt.Errorf("cannot create zip file: %v", err)
	}
	defer zipFile.Close()

	// 創建 zip.Writer 寫入 ZIP 文件
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// 創建 JSON 文件部分並添加到 ZIP 文件
	jsonFile, err := zipWriter.Create("metadata.json")
	if err != nil {
		return err
	}

	//
	jsonBytes, err := json.MarshalIndent(result, "", "	")
	if err != nil {
		return err
	}

	//
	_, err = jsonFile.Write(jsonBytes)
	if err != nil {
		return err
	}

	// 遍歷查詢結果中的圖片名稱，並添加到 ZIP 檔案
	for _, image := range result {

		// 路徑
		imagePath := filepath.Join(templates.GlobalRootPath, image.PhotoName)
		//log.Printf("名稱:%s", imagePath)

		// 開啟圖片檔案
		imageFile, err := os.Open(imagePath)
		if err != nil {
			return fmt.Errorf("無法開啟圖片檔案 %s: %v", image.PhotoName, err)
		}
		defer imageFile.Close()

		// 創建 ZIP 檔案內部條目
		zipEntryWriter, err := zipWriter.Create(image.PhotoName)
		if err != nil {
			return fmt.Errorf("無法創建 ZIP 條目: %v", err)
		}

		// 複製圖片內容到 ZIP
		_, err = io.Copy(zipEntryWriter, imageFile)
		if err != nil {
			return fmt.Errorf("無法寫入圖片檔案到 ZIP: %v", err)
		}
	}

	return nil
}
