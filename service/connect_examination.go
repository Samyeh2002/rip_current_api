package service

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// 測試 1
func Connect_test_get(c *gin.Context) {

	filename := c.Param("url")
	imagePath := filepath.Join("C:\\Users\\Aldronius\\Documents", filename)

	fileInfo2, err2 := os.Stat(imagePath)
	if err2 != nil {
		// 發生錯誤
		fmt.Println("無法獲取檔案元資料:", err2)
		return
	}

	log.Printf("zip目標檔案大小為:%d", fileInfo2.Size())

	c.File(imagePath)
}

// 測試 2
func Connect_test_get_other(c *gin.Context) {

	// 資料夾路徑
	dir := "C:\\Users\\Aldronius\\Documents\\Rip_Current_Photo"

	// 創建緩衝區
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// 遍歷資料夾中的所有文件和子資料夾
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 創建文件項目
		zipFile, err := zipWriter.Create(path)
		if err != nil {
			return err
		}

		// 如果是文件，複製文件內容到 ZIP 中
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(zipFile, file)
			if err != nil {
				return err
			}
		}
		return nil
	})

	// 關閉 ZIP 寫入器
	err := zipWriter.Close()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// 設置 Content-Type 頭部
	c.Header("Content-Type", "application/zip")
	// 設置 Content-Disposition 頭部
	c.Header("Content-Disposition", "attachment; filename=files.zip")

	// 將 ZIP 數據寫入響應主體中
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}

// 測試 3(獲得圖片)
func Connect_test_get_check(c *gin.Context) {

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

	// 構建圖片文件的絕對路徑
	imagePath := filepath.Join("C:\\Users\\Aldronius\\Documents\\Rip_Current_Photo", "www.png")

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

// 測試 4 (取得請求的完整資訊)
func Connect_test_Error(c *gin.Context) {

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
}
