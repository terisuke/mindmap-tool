package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Node はマインドマップのノードを表す構造体
type Node struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	MapID     uint      `json:"map_id"`
	Text      string    `json:"text"`
	ParentID  *uint     `json:"parent_id"`
	Position  string    `json:"position"` // JSON文字列として位置情報を保存
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// 接続関連のフィールド
	FromConnections []NodeConnection `gorm:"foreignKey:FromNodeID" json:"from_connections"`
	ToConnections   []NodeConnection `gorm:"foreignKey:ToNodeID" json:"to_connections"`
}

// NodeConnection はノード間の接続を表す構造体
type NodeConnection struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	FromNodeID uint      `json:"from_node_id"`
	ToNodeID   uint      `json:"to_node_id"`
	Label      string    `json:"label"` // 接続線のラベル（オプション）
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// MindMap はマインドマップの全体情報を表す構造体
type MindMap struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Title     string    `json:"title"`
	UserID    uint      `json:"user_id"`
	Nodes     []Node    `gorm:"foreignKey:MapID" json:"nodes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// User はユーザー情報を表す構造体
type User struct {
	ID       uint      `gorm:"primaryKey" json:"id"`
	Username string    `json:"username"`
	Password string    `json:"-"` // JSONではパスワードを出力しない
	MindMaps []MindMap `gorm:"foreignKey:UserID" json:"mind_maps"`
}

var db *gorm.DB

func initDB() {
	var err error

	// MySQLコンテナの設定に合わせてユーザ名・パスワードを調整
	// 例: root:password で指定
	dsn := "root:password@tcp(127.0.0.1:3306)/mindmap_db?charset=utf8mb4&parseTime=True&loc=Local"

	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("データベース接続エラー:", err)
	}

	// マイグレーション: テーブルを自動作成/更新
	db.AutoMigrate(&User{}, &MindMap{}, &Node{}, &NodeConnection{})

	// テストユーザーの作成
	var count int64
	db.Model(&User{}).Count(&count)
	if count == 0 {
		testUser := User{
			Username: "test_user",
			Password: "test_password", // 実際のアプリではハッシュ化する必要があります
		}
		db.Create(&testUser)
		log.Println("テストユーザーを作成しました")
	}
}

func main() {
	initDB()

	r := gin.Default()

	// CORSミドルウェアを設定: 必要に応じてAllowOriginsを変更
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:8080"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 静的ファイルの提供 (/static/ というURLで ./static フォルダを参照)
	r.Static("/static", "./static")
	// テンプレートの読み込み (templates/ 以下を全部読み込み)
	r.LoadHTMLGlob("templates/*")

	// ルートページ ("/") で index.html を表示
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "マインドマップツール",
		})
	})

	// APIグループ
	api := r.Group("/api")
	{
		// マインドマップ関連のエンドポイント
		api.GET("/mindmaps", getAllMindMaps)
		api.GET("/mindmaps/:id", getMindMap)
		api.POST("/mindmaps", createMindMap)
		api.PUT("/mindmaps/:id", updateMindMap)
		api.DELETE("/mindmaps/:id", deleteMindMap)

		// ノード関連のエンドポイント
		api.POST("/nodes", createNode)
		api.PUT("/nodes/:id", updateNode)
		api.DELETE("/nodes/:id", deleteNode)

		// 接続関連のエンドポイント
		api.POST("/connections", createConnection)
		api.DELETE("/connections/:id", deleteConnection)
	}

	// ポート8080番でサーバ起動
	r.Run(":8080")
}

// すべてのマインドマップを取得
func getAllMindMaps(c *gin.Context) {
	var mindmaps []MindMap
	result := db.Find(&mindmaps)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, mindmaps)
}

// 特定のマインドマップを取得 (関連するNodeも含める)
func getMindMap(c *gin.Context) {
	id := c.Param("id")
	var mindmap MindMap
	result := db.Preload("Nodes").
		Preload("Nodes.FromConnections").
		Preload("Nodes.ToConnections").
		First(&mindmap, id)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "マインドマップが見つかりません"})
		return
	}
	c.JSON(http.StatusOK, mindmap)
}

// 新しいマインドマップを作成
func createMindMap(c *gin.Context) {
	var mindmap MindMap
	if err := c.ShouldBindJSON(&mindmap); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := db.Create(&mindmap)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusCreated, mindmap)
}

// マインドマップを更新
func updateMindMap(c *gin.Context) {
	id := c.Param("id")
	var mindmap MindMap
	result := db.First(&mindmap, id)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "マインドマップが見つかりません"})
		return
	}

	if err := c.ShouldBindJSON(&mindmap); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db.Save(&mindmap)
	c.JSON(http.StatusOK, mindmap)
}

// マインドマップを削除 (関連するノードも同時に削除)
func deleteMindMap(c *gin.Context) {
	id := c.Param("id")
	var mindmap MindMap
	result := db.First(&mindmap, id)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "マインドマップが見つかりません"})
		return
	}

	// 関連するノードも削除
	db.Where("map_id = ?", id).Delete(&Node{})
	db.Delete(&mindmap)

	c.JSON(http.StatusOK, gin.H{"message": "マインドマップが削除されました"})
}

// 新しいノードを作成
func createNode(c *gin.Context) {
	var node Node
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := db.Create(&node)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusCreated, node)
}

// ノードを更新
func updateNode(c *gin.Context) {
	id := c.Param("id")
	var node Node
	result := db.First(&node, id)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ノードが見つかりません"})
		return
	}

	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db.Save(&node)
	c.JSON(http.StatusOK, node)
}

// ノードを削除
func deleteNode(c *gin.Context) {
	id := c.Param("id")
	var node Node
	result := db.First(&node, id)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ノードが見つかりません"})
		return
	}

	db.Delete(&node)
	c.JSON(http.StatusOK, gin.H{"message": "ノードが削除されました"})
}

// 新しい接続を作成
func createConnection(c *gin.Context) {
	var connection NodeConnection
	if err := c.ShouldBindJSON(&connection); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := db.Create(&connection)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusCreated, connection)
}

// 接続を削除
func deleteConnection(c *gin.Context) {
	id := c.Param("id")
	var connection NodeConnection
	result := db.First(&connection, id)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "接続が見つかりません"})
		return
	}

	db.Delete(&connection)
	c.JSON(http.StatusOK, gin.H{"message": "接続が削除されました"})
}
