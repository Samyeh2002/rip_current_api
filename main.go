package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http/httputil"
	"net/url"
	"os"
	"rip_current_mod/database"
	"rip_current_mod/routes"
	"rip_current_mod/service"
	"rip_current_mod/templates"
	"time"

	//"github.com/gin-contrib/cors"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

var Texx string = "dd"

// 主程式
func main() {
	router := gin.Default()

	// 啟動資料庫
	go func() {
		database.Ms()
		// database.Redis_initialization()
		go service.SchedulePermissionUpdates()
		go service.ScheduleOTPUpdates()
		go service.Member_ScheduleOTPUpdates()
		go service.Web_update_complete_photo()
		go service.Web_update_load_photo()
		service.Web_Schedule_photo()

	}()

	// 檢查資料夾(圖片)
	go func() {
		Folder_check()
	}()

	// 建立反向代理人
	// reverseProxy := func(c *gin.Context) {
	// 	targetURL := "https://192.168.50.159" // 目标服务器的地址192.168.50.159
	// 	proxy := NewSingleHostReverseProxy(targetURL)
	// 	proxy.ServeHTTP(c.Writer, c.Request)
	// }

	// 設定cors
	router.Use(cors.New(cors.Config{
		//AllowAllOrigins: true,
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "X-Requested-With", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	router.Use(cors.Default())

	//  創建路由組
	basic_path := router.Group("rip_current")
	routes.Path_set(basic_path)

	//
	router.NoRoute(service.Connect_test_Error)

	// 將反向代理人加入到路徑
	//router.Any("/", reverseProxy)

	// 生成一對 RSA 私鑰和一個自簽名憑證
	//Private_key()

	// 啟動路由引擎
	router.Run("192.168.1.223:2234") // 192.168.1.223:2234 192.168.50.29
}

// 反向代理人
func NewSingleHostReverseProxy(target string) *httputil.ReverseProxy {
	targetURL, _ := url.Parse(target)
	return httputil.NewSingleHostReverseProxy(targetURL)
}

// 檢查和新增存放圖片的資料夾
func Folder_check() {

	// 變數
	folderPath := templates.GlobalRootPath

	// 使用 Mkdir 函數建立資料夾 #0755 是資料夾的權限設定，表示可讀可寫可執行
	if err := os.Mkdir(folderPath, 0755); err != nil {

		// 新增失敗 (資料夾已經存在)
		fmt.Println("建立資料夾失敗:", err)
		return
	}

	// 新增成功
	fmt.Println("資料夾建立成功")
}

// 生成一對 RSA 私鑰和一個自簽名憑證
func Private_key() {

	// 產生 RSA 私鑰
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Println("Failed to generate private key:", err)
		return
	}

	privateKeyFile, err := os.Create("server.key")
	if err != nil {
		fmt.Println("Failed to create private key file:", err)
		return
	}
	defer privateKeyFile.Close()

	err = pem.Encode(privateKeyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	if err != nil {
		fmt.Println("Failed to encode and write private key:", err)
		return
	}

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		BasicConstraintsValid: true,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certificateDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		fmt.Println("Failed to create certificate:", err)
		return
	}

	certificateFile, err := os.Create("server.crt")
	if err != nil {
		fmt.Println("Failed to create certificate file:", err)
		return
	}
	defer certificateFile.Close()

	err = pem.Encode(certificateFile, &pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	if err != nil {
		fmt.Println("Failed to encode and write certificate:", err)
		return
	}

	fmt.Println("SSL/TLS certificate and private key generated successfully.")
}

// err := router.RunTLS("", "server.crt", "server.key")
// if err != nil {
// 	log.Fatal("Failed to start HTTPS server: ", err)
// }
