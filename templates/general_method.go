package templates

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// 簡介: 收錄一切通用的方法來處理小問題或檢查

func Http_information_display(target *http.Request) {
	// 備份請求體
	var bodyBuffer bytes.Buffer
	bodyBuffer.ReadFrom(target.Body)

	// 重設 Body，使其可以再次讀取
	target.Body = io.NopCloser(bytes.NewReader(bodyBuffer.Bytes()))

	// 輸出請求方法
	fmt.Println("Method:", target.Method)

	// 輸出請求 URL
	fmt.Println("URL:", target.URL.String())

	// 輸出請求標頭
	fmt.Println("Headers:")
	for name, values := range target.Header {
		for _, value := range values {
			fmt.Println(name+":", value)
		}
	}

	// 輸出請求體長度
	fmt.Println("Body Length:", target.ContentLength)

	// 如果是 multipart/form-data，則解析每個部分
	if strings.Contains(target.Header.Get("Content-Type"), "multipart/form-data") {
		// 重設 Body 再讀取
		target.Body = io.NopCloser(bytes.NewReader(bodyBuffer.Bytes()))

		reader, err := target.MultipartReader()
		if err != nil {
			fmt.Println("Error reading multipart form:", err)
			return
		}

		// 解析每個部分
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Println("Error reading part:", err)
				return
			}

			// 輸出車廂名稱 (部分名稱)
			fmt.Println("Part Name:", part.FormName())

			// 如果是文件，則輸出文件名
			if part.FileName() != "" {
				fmt.Println("File Name:", part.FileName())
			} else {
				// 輸出表單字段內容
				body, _ := io.ReadAll(part)
				fmt.Println("Form Field Value:", string(body))
			}
		}
	} else {
		fmt.Println("This is not a multipart/form-data request.")
	}

	// 重設 Body，使其可以被後續操作再次讀取
	target.Body = io.NopCloser(bytes.NewReader(bodyBuffer.Bytes()))
}

// 檢查圖檔是否符合指定類型並檢查是否有效
func Picture_file_check(file multipart.File) (string, bool) {

	// 符合的範圍
	var allowedFileTypes = map[string]string{
		"image/jpeg": "jpg",
		"image/png":  "png",
	}

	// 讀取檔案的前512位元組來判斷檔案的MIME類型
	buff := make([]byte, 512)
	if _, err := file.Read(buff); err != nil {
		return "", false
	}

	// 取得檔案的MIME類型
	fileType := http.DetectContentType(buff)

	// 將檔案指標重設到開頭，以便後續操作
	file.Seek(0, 0)

	// 檢查文件類型是否允許
	ext, isValid := allowedFileTypes[fileType]
	return ext, isValid
}

// 方法3-計算Haversine的公式
func Haversine(lat1, lon1, lat2, lon2 float64) float64 {

	// 變數
	const radius = 6371 // 地球半徑（公里）
	deltaLat := (lat2 - lat1) * (math.Pi / 180.0)
	deltaLon := (lon2 - lon1) * (math.Pi / 180.0)

	// 公式
	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1*(math.Pi/180.0))*math.Cos(lat2*(math.Pi/180.0))*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distance := radius * c

	// 回傳
	return distance
}

// 用來將String轉成Float64
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

// 歷遍資料夾 並將資料回傳 接收2個參數(資料夾位置、*gin.Context)
func FolderTraverseAndReturn(location_target string, parameter_target *gin.Context) {

	//變數
	location := location_target
	parameter := parameter_target

	// 創建一個內存中的 ZIP 壓縮文件
	parameter.Writer.Header().Set("Content-Disposition", "attachment; filename=images.zip")
	parameter.Writer.Header().Set("Content-Type", "application/zip")

	zipWriter := zip.NewWriter(parameter.Writer)
	defer zipWriter.Close()

	// 遍歷資料夾，將圖片加入 ZIP 檔案
	err := filepath.Walk(location, func(path string, info os.FileInfo, err error) error {
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
		parameter.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create ZIP file: %v", err))
		return
	}
}

// 根據區域碼轉換成路徑
func GetAreaPath(rootPath string, code string) string {

	// 目標為空

	// 建立一個代號與路徑的映射
	pathMap := map[string]string{
		"海灘A":   filepath.Join(rootPath, code),
		"code2": "/path/to/resource2",
		"code3": "/path/to/resource3",
		"null":  rootPath,
	}

	// 尋找對應的路徑
	if path, exists := pathMap[code]; exists {
		return path
	}

	// 如果代號不存在，則傳回一個預設的路徑或錯誤訊息
	return "路徑未找到"
}
