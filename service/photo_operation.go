package service

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"rip_current_mod/database"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// 相片資訊
type InformationStruct struct {
	PhotoName           string `gorm:"column:photoName;default:null"`
	PhotoLocation       string `gorm:"column:photoLocation;default:null"`
	PhotoCoordinate_lng string `gorm:"column:photoCoordinate_lng;default:null"`
	PhotoCoordinate_lat string `gorm:"column:photoCoordinate_lat;default:null"`
	PhotoFilming_time   string `gorm:"column:photoFilming_time;default:null"`
	PhotoPosition       string `gorm:"column:photoPosition;default:null"`
	LikeQuantity        int    `gorm:"column:likeQuantity"`
	IsLike              bool   `gorm:"-"`
}

// 獲取圖片_多張_使用GET (ver.1 完成)
func Photo_operation_get_folder(c *gin.Context) {
	// 變數-路徑
	folderToZip := "C:\\Users\\Aldronius\\Documents\\Rip_Current_Photo" // 要壓縮的資料夾路徑
	zipFilePath := "C:\\Users\\Aldronius\\Documents\\output.zip"        // zip檔案
	var zipFile *os.File
	var err error

	// 使用通道來等待 goroutine 完成
	done := make(chan bool)

	go func() {
		defer close(done)

		// 確認目標資料夾存在
		if _, err := os.Stat(folderToZip); os.IsNotExist(err) {
			c.String(http.StatusInternalServerError, "資料夾不存在: %v", err)
			return
		}

		// 創建 ZIP 檔案
		zipFile, err = os.Create(zipFilePath)
		if err != nil {
			fmt.Println("無法建立 ZIP 檔案:", err)
			c.String(http.StatusInternalServerError, "無法建立 ZIP 檔案")
			return
		}
		defer zipFile.Close()

		// 建立 zipWriter
		zipWriter := zip.NewWriter(zipFile)
		defer zipWriter.Close()

		// 將資料夾壓成zip
		err := filepath.Walk(folderToZip, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// 取得相對路徑
			relPath, err := filepath.Rel(folderToZip, path)
			if err != nil {
				return err
			}

			// 如果是資料夾，建立目錄
			if info.IsDir() {
				return nil
			}

			// 創建 ZIP 中的檔案
			fileInZip, err := zipWriter.Create(relPath)
			if err != nil {
				return err
			}

			// 打開檔案
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			// 複製檔案內容到 ZIP 中
			_, err = io.Copy(fileInZip, file)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			fmt.Println("建立 ZIP 檔案失敗:", err)
			return
		}

		fmt.Println("ZIP 檔案建立成功:", zipFilePath)
	}()

	// 等待 goroutine 完成
	<-done

	// 設定 HTTP 響應標頭
	c.Writer.Header().Set("Content-Type", "application/zip")
	c.Writer.Header().Set("Content-Disposition", "attachment; filename=test_6.zip")

	// 讀取 ZIP 檔案並回傳給客戶端
	c.File(zipFilePath)
	// buffer := make([]byte, 1024) // 每個分段的大小為 1024 bytes
	// for {
	// 	n, err := zipFile.Read(buffer)
	// 	if err != nil && err != io.EOF {
	// 		fmt.Println("讀取 ZIP 檔案失敗:", err)
	// 		c.JSON(http.StatusInternalServerError, gin.H{"message": "讀取 ZIP 檔案失敗"})
	// 		return
	// 	}
	// 	if n == 0 {
	// 		break
	// 	}
	// 	if _, err := c.Writer.Write(buffer[:n]); err != nil {
	// 		fmt.Println("寫入 HTTP 回應失敗:", err)
	// 		return
	// 	}
	// }
}

// 獲取圖片資訊_多張_使用GET (ver.1 測試中)
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
			log.Printf("%s的距離為:%.2f公尺", i.PhotoName, haversine(isLogin.UserLatitude, isLogin.UserLongitude, StringChangeFloat64(i.PhotoCoordinate_lat), StringChangeFloat64(i.PhotoCoordinate_lng))*1000)
		}
		slices.SortFunc(users, func(a, b InformationStruct) int {

			// 計算距離
			dist1 := haversine(isLogin.UserLatitude, isLogin.UserLongitude, StringChangeFloat64(a.PhotoCoordinate_lat), StringChangeFloat64(a.PhotoCoordinate_lng)) // 緯度 經度
			dist2 := haversine(isLogin.UserLatitude, isLogin.UserLongitude, StringChangeFloat64(b.PhotoCoordinate_lat), StringChangeFloat64(b.PhotoCoordinate_lng)) // 緯度 經度

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
			dist1 := haversine(isLogin.UserLatitude, isLogin.UserLongitude, StringChangeFloat64(a.PhotoCoordinate_lat), StringChangeFloat64(a.PhotoCoordinate_lng)) // 緯度 經度
			dist2 := haversine(isLogin.UserLatitude, isLogin.UserLongitude, StringChangeFloat64(b.PhotoCoordinate_lat), StringChangeFloat64(b.PhotoCoordinate_lng)) // 緯度 經度

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

// 獲取圖片_單張_使用GET (ver.1 完成 )
func Photo_operation_get_one(c *gin.Context) {

	// 變數
	filename := c.Param("filename")

	// 構建圖片文件的絕對路徑
	imagePath := filepath.Join("C:\\Users\\Aldronius\\Documents\\Rip_Current_Photo", filename)

	// 打開圖片文件
	file, err := os.Open(imagePath)
	if err != nil {
		c.JSON(404, gin.H{"message": fmt.Sprintf("圖片 %s 不存在", imagePath)})
		return
	}
	defer file.Close()

	// 設置 Content-Type 頭部
	c.Header("Content-Type", "image/jpeg") // 這裡假設圖片是 JPEG 格式，根據實際情況修改

	// 將圖片複製到響應主體中
	io.Copy(c.Writer, file)
}

// 獲取圖片資訊_使用GET (ver.1 完成 ) #輸入圖片名稱(要加.JPG) 來獲得圖片資訊
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

// 獲取圖片_單張_使用GET_包含圖片資訊 (ver.1 完成 )
func Photo_operation_get_one_include_information(c *gin.Context) {
	// 變數
	filename := c.Param("filename")

	// 構建圖片文件的絕對路徑
	imagePath := filepath.Join("C:\\Users\\Aldronius\\Documents\\Rip_Current_Photo", filename)

	// 打開圖片文件
	file, err := os.Open(imagePath)
	if err != nil {
		c.JSON(404, gin.H{"message": fmt.Sprintf("圖片 %s 不存在", imagePath)})
		return
	}
	defer file.Close()

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
	c.Header("Content-Type", "multipart/mixed; boundary=BOUNDARY")

	writer := multipart.NewWriter(c.Writer)
	writer.SetBoundary("BOUNDARY")

	// 寫入 JSON 部分
	jsonPartHeader := make(textproto.MIMEHeader)
	jsonPartHeader.Set("Content-Type", "application/json")
	jsonPartHeader.Set("Content-Disposition", `form-data; name="information"`)
	jsonPart, err := writer.CreatePart(jsonPartHeader)
	if err != nil {
		c.JSON(500, gin.H{"message": "無法創建 JSON 部分"})
		return
	}
	jsonPart.Write(jsonData)

	// 寫入圖片部分
	imagePartHeader := make(textproto.MIMEHeader)
	imagePartHeader.Set("Content-Type", "image/jpeg") // 根據實際情況修改 Content-Type
	imagePartHeader.Set("Content-Disposition", `form-data; name="image"; filename="`+filename+`"`)
	imagePart, err := writer.CreatePart(imagePartHeader)
	if err != nil {
		c.JSON(500, gin.H{"message": "無法創建圖片部分"})
		return
	}
	imagePart.Write(imageBytes)

	// 關閉 writer
	writer.Close()
}

// 新增圖片_多張_使用POST (ver.1 測試中)
func Photo_operation_post_folder(c *gin.Context) {

}

// 新增圖片_單張_使用POST (ver.2 完成 現在可以新增圖片時添加圖片資訊) #需要5個參數
func Photo_operation_post_one(c *gin.Context) {

	// 變數
	storePath := "C:\\Users\\Aldronius\\Documents\\Rip_Current_Photo"
	photoInfo := InformationStruct{}
	var wg sync.WaitGroup

	// 獲取請求對象
	req := c.Request

	// 輸出請求方法
	fmt.Println("Method:", req.Method)

	// 輸出請求 URL
	fmt.Println("URL:", req.URL.String())

	// 輸出請求標頭
	fmt.Println("Headers:")
	for name, values := range req.Header {
		for _, value := range values {
			fmt.Println(name+":", value)
		}
	}

	// 輸出請求體（在這個示例中只是打印請求體的長度）
	fmt.Println("Body Length:", req.ContentLength)

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
	ext, isValid := getFileType(file)
	if !isValid {
		c.JSON(http.StatusBadRequest, gin.H{"message": "不支持的文件类型"})
		return
	}

	// 視情況添加副檔名
	if !strings.Contains(handler.Filename, ".") {

		//
		photoInfo.PhotoName = fmt.Sprintf("%s.%s", handler.Filename, ext)
	} else {

		// 按照原定計畫
		photoInfo.PhotoName = handler.Filename
	}

	// 計數
	wg.Add(1)

	go func() {

		// 額外任務
		defer wg.Done()

		// 創建目標文件
		dst, err := os.Create(filepath.Join(storePath, photoInfo.PhotoName))
		if err != nil {
			log.Print("無法創建目標文件")
			c.JSON(500, gin.H{"message": "無法創建目標文件"})
			return
		}
		defer dst.Close()

		// 複製文件內容到目標文件
		_, err = io.Copy(dst, file)
		if err != nil {
			log.Print("無法複製文件內容")
			c.JSON(500, gin.H{"message": "無法複製文件內容"})
			return
		}
	}()

	// 擷取資訊
	info := c.PostForm("information")
	if err := json.Unmarshal([]byte(info), &photoInfo); err != nil {
		log.Printf("Invalid JSON: %s", err)
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid JSON"})
		return
	}

	// 測試
	fmt.Printf("%s", info)

	// 新增到資料庫
	if err := database.MyConnect.Table("rip_current_information").Create(&photoInfo).Error; err != nil {
		log.Print("新增到資料庫失敗")
		c.JSON(200, gin.H{"message": "新增圖片資料到資料庫失敗"})
		return
	}

	// 成功
	log.Printf("圖片:%s的資訊新增到資料庫成功", photoInfo.PhotoName)

	wg.Wait()

	// 回傳
	c.JSON(200, gin.H{"message": "文件上傳成功"})
}

// 定義允許的文件類型
var allowedFileTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
}

// 获取文件类型并检查是否有效
func getFileType(file multipart.File) (string, bool) {
	// 读取文件的前512字节来判断文件的MIME类型
	buff := make([]byte, 512)
	if _, err := file.Read(buff); err != nil {
		return "", false
	}

	// 获取文件的MIME类型
	fileType := http.DetectContentType(buff)

	// 将文件指针重置到开头，以便后续操作
	file.Seek(0, 0)

	// 检查文件类型是否允许
	ext, isValid := allowedFileTypes[fileType]
	return ext, isValid
}

// 計算Haversine的公式
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const radius = 6371 // Earth radius in kilometers
	deltaLat := (lat2 - lat1) * (math.Pi / 180.0)
	deltaLon := (lon2 - lon1) * (math.Pi / 180.0)

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1*(math.Pi/180.0))*math.Cos(lat2*(math.Pi/180.0))*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	distance := radius * c
	return distance
}

func StringChangeFloat64(target string) float64 {

	// 變數
	var ans float64
	var err error

	// 轉換
	if ans, err = strconv.ParseFloat(target, 64); err != nil {
		return 0
	}

	// 回傳
	return ans
}
