package service

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"rip_current_mod/templates"

	"github.com/gin-gonic/gin"
)

// 結構

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

// 測試:5 調用Python
func Call_Python(c *gin.Context) {

	// 變數
	interpreter := "C:/Users/PC3/AppData/Local/Programs/Python/Python313/python.exe"
	execution_File := "C:/Users/PC3/Documents/Python/test2.py"
	target := c.DefaultQuery("path", "null")

	log.Printf("測試:%v", target)

	cmd := exec.Command(interpreter, execution_File, filepath.Join(templates.GlobalRootPath, target))

	// 執行並獲取輸出--
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Println("執行 Python 腳本時出錯1:", err)
		log.Println("執行 Python 腳本時出錯2:", output)
		c.JSON(400, gin.H{"message": fmt.Sprintf("失敗 路徑:%v", filepath.Join(templates.GlobalRootPath, target))})
		return
	}

	// --
	fmt.Println("Python 輸出:", string(output))
}

// 測試:5 調用Python 掃圖在回傳
func Call_Python_Return() {

}

// 測試:6 Gmail
func SendGmailTest(c *gin.Context) {

	//SendGmail()
	log.Printf("用戶IP:%v", c.ClientIP())
}

// 測試參數
func SyncTest(c *gin.Context) {

	// 變數
	Target := c.DefaultQuery("mod", "")
	var count int64

	switch Target {
	case "1": // OperationCache 測試

		templates.UserOTPCache.Range(func(key, value any) bool {

			// 累加
			count++

			// 取值
			if testValue, ok := value.(UserOTPstruct); !ok {

				// 錯誤 非目標
				return true

			} else {

				// 輸出
				log.Printf("目標:%v 金鑰:%v 驗證碼:%v 插入時間:%v", testValue.UserID, testValue.UserOTPsecret, testValue.VerificationCode, testValue.InsertTime)
				return true
			}
		})

		// 輸出有幾筆數據
		c.JSON(200, fmt.Sprintf("總共有:%v", count))

	case "2": // UserOTPCache測試
	case "3": // PasswordForgotCache測試
	default:
		log.Print("Sync測試沒給參數")
	}

}
