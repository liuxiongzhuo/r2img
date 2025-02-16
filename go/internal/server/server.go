package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"

	"r2img/internal/config"
	"r2img/internal/image"
	"r2img/internal/platform" // 导入 platform
)

// Server 是一个自定义的 HTTP 服务器。
type Server struct {
	config *config.Config
}

// NewServer 创建一个新的 Server 实例。
func NewServer(cfg *config.Config) *Server {
	return &Server{
		config: cfg,
	}
}

// ServeHTTP 实现了 http.Handler 接口。
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/upload" && r.Method == http.MethodPost:
		s.handleUpload(w, r)
	case strings.HasPrefix(r.URL.Path, "/i/"):
		s.handleImage(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {

	// 检查认证
	auth := r.Header.Get("Authorization")
	if auth != "Bearer "+s.config.AuthKey {
		log.Printf("认证失败: 无效的授权头\n")
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}

	// 限制上传文件大小
	r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxFileSize*1024*1024) // 限制上传大小
	err := r.ParseMultipartForm(s.config.MaxFileSize * 1024 * 1024)
	if err != nil {
		log.Printf("解析多部分表单失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}

	defer r.MultipartForm.RemoveAll() // 清理上传的文件

	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("获取上传文件失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	uploadedFilename, err := image.HandleFileUpload(file, header, s.config) // 使用 image 包
	if err != nil {
		log.Printf("处理文件上传时出错：%v", err)                          // 记录具体错误
		http.Error(w, "服务器内部错误", http.StatusInternalServerError) // 返回 500 错误
		return
	}

	w.WriteHeader(http.StatusOK)    // 设置状态码为 200 OK
	fmt.Fprint(w, uploadedFilename) // 返回上传后的文件名

}

var cleanMutex sync.Mutex

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {

	// 获取 filename
	filename := strings.TrimPrefix(r.URL.Path, "/i/")

	// 检查 filename 是否为空
	if filename == "" {
		log.Println("请求的 filename 为空")
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}

	// 构造文件路径
	filePath := "./i/" + filename

	// 检查本地是否存在
	if _, err := os.Stat(filePath); err == nil {
		// 存在，直接返回
		http.ServeFile(w, r, filePath)
		return
	}

	// 不存在，请求远程 URL
	remoteURL := s.config.ApiSite + "/i/" + filename
	req, err := http.NewRequest("GET", remoteURL, nil)
	if err != nil {
		log.Printf("创建请求失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "Bearer "+s.config.ApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("请求远程资源失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("远程资源返回状态码: %d\n", resp.StatusCode)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}

	// 读取响应内容
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("读取远程内容失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}

	// 保存到本地
	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		if perr, ok := err.(*os.PathError); ok && perr.Err == syscall.ENOSPC {
			// ENOSPC: No space left on device (磁盘空间不足)
			log.Printf("保存文件失败: 磁盘空间不足: %v\n", err)
			// 尝试清理空间 (在 goroutine 中)
			go func() {
				cleanMutex.Lock() // 加锁,防止多个清理同时进行
				defer cleanMutex.Unlock()

				log.Println("开始清理旧文件...")
				cleanErr := platform.CleanOldFiles("./i", s.config.MaxCacheSize*1024*1024, s.config.FreeCacheSize*1024*1024)
				if cleanErr != nil {
					log.Printf("清理旧文件失败: %v\n", cleanErr)
				} else {
					log.Println("清理旧文件完成。")
				}
			}()

			http.Error(w, "磁盘空间不足", http.StatusInsufficientStorage) // 507 Insufficient Storage
		} else {
			log.Printf("保存文件失败: %v\n", err)
			http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		}
		return
	}

	// 返回文件
	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Write(data)
}
