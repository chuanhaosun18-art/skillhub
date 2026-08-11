package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// ---------- 查询 ----------

// listSkills GET /api/skills?keyword=&category=&sort=
func listSkills(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	category := strings.TrimSpace(c.Query("category"))
	sortBy := c.DefaultQuery("sort", "newest")

	where := []string{}
	args := []interface{}{}

	if keyword != "" {
		where = append(where, "(name LIKE ? OR description LIKE ? OR category LIKE ? OR tags LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like, like, like)
	}
	if category != "" && category != "全部" {
		where = append(where, "category = ?")
		args = append(args, category)
	}

	order := "s.created_at DESC"
	switch sortBy {
	case "rating":
		order = "s.rating DESC, s.created_at DESC"
	case "downloads":
		order = "s.download_count DESC, s.created_at DESC"
	case "oldest":
		order = "s.created_at ASC"
	}

	query := `SELECT s.id, s.owner_id, COALESCE(u.username,''), s.name, s.description,
		s.category, s.tags, s.version, s.icon, s.file_count, s.total_size,
		s.download_count, s.view_count, s.rating, s.rating_count, s.created_at, s.updated_at
		FROM skills s LEFT JOIN users u ON s.owner_id = u.id`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY " + order

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	skills := []Skill{}
	for rows.Next() {
		var s Skill
		if err := rows.Scan(&s.ID, &s.OwnerID, &s.OwnerName, &s.Name, &s.Description,
			&s.Category, &s.Tags, &s.Version, &s.Icon, &s.FileCount, &s.TotalSize,
			&s.DownloadCount, &s.ViewCount, &s.Rating, &s.RatingCount, &s.CreatedAt, &s.UpdatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		skills = append(skills, s)
	}
	c.JSON(http.StatusOK, gin.H{"data": skills, "total": len(skills)})
}

// getSkill GET /api/skills/:id
func getSkill(c *gin.Context) {
	id := c.Param("id")

	var s Skill
	err := db.QueryRow(`SELECT s.id, s.owner_id, COALESCE(u.username,''), s.name, s.description,
		s.category, s.tags, s.version, s.icon, s.file_count, s.total_size,
		s.download_count, s.view_count, s.rating, s.rating_count, s.created_at, s.updated_at
		FROM skills s LEFT JOIN users u ON s.owner_id = u.id WHERE s.id = ?`, id).
		Scan(&s.ID, &s.OwnerID, &s.OwnerName, &s.Name, &s.Description,
			&s.Category, &s.Tags, &s.Version, &s.Icon, &s.FileCount, &s.TotalSize,
			&s.DownloadCount, &s.ViewCount, &s.Rating, &s.RatingCount, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
		return
	}

	// 文件列表
	rows, err := db.Query(`SELECT id, skill_id, file_path, size, sha256 FROM skill_files WHERE skill_id = ? ORDER BY file_path`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var f SkillFile
			if err := rows.Scan(&f.ID, &f.SkillID, &f.FilePath, &f.Size, &f.SHA256); err == nil {
				s.Files = append(s.Files, f)
			}
		}
	}

	// 浏览量 +1
	db.Exec(`UPDATE skills SET view_count = view_count + 1 WHERE id = ?`, id)

	c.JSON(http.StatusOK, gin.H{"data": s})
}

// ---------- 创建 ----------

// createSkill POST /api/skills (multipart/form-data, 需登录)
// 字段: name, description, category, tags(JSON数组), version
// 文件: archive (zip)
func createSkill(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	description := c.PostForm("description")
	category := c.PostForm("category")
	tags := c.PostForm("tags")
	if tags == "" {
		tags = "[]"
	} else if !json.Valid([]byte(tags)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tags must be a JSON array string"})
		return
	}
	version := c.PostForm("version")
	if version == "" {
		version = "1.0.0"
	}
	// 发布者：来自登录 token
	ownerID := c.GetInt64("userID")

	// 创建 skill 记录
	result, err := db.Exec(`INSERT INTO skills (owner_id, name, description, category, tags, version) VALUES (?, ?, ?, ?, ?, ?)`,
		ownerID, name, description, category, tags, version)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	skillID, _ := result.LastInsertId()

	// 处理上传的 zip 包
	archive, err := c.FormFile("archive")
	if err == nil {
		if err := saveAndExtractArchive(c, skillID, archive); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "save archive failed: " + err.Error()})
			return
		}
	}

	skill, err := getSkillByID(skillID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": skill})
}

// saveAndExtractArchive 保存 zip 并解压、登记文件清单
func saveAndExtractArchive(c *gin.Context, skillID int64, archive *multipart.FileHeader) error {
	skillDir := filepath.Join(FilesDir, fmt.Sprintf("%d", skillID))
	os.MkdirAll(skillDir, 0o755)

	// 保存原始 zip
	archivePath := filepath.Join(ArchiveDir, fmt.Sprintf("%d.zip", skillID))
	src, err := archive.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	dst.Close()

	// 解压
	if err := extractZip(archivePath, skillDir); err != nil {
		return err
	}

	// 登记文件清单
	files, err := indexFiles(skillID, skillDir)
	if err != nil {
		return err
	}

	var totalSize int64
	for _, f := range files {
		if _, err := db.Exec(`INSERT INTO skill_files (skill_id, file_path, size, sha256) VALUES (?, ?, ?, ?)`,
			skillID, f.FilePath, f.Size, f.SHA256); err != nil {
			return err
		}
		totalSize += f.Size
	}

	_, err = db.Exec(`UPDATE skills SET file_count = ?, total_size = ?, archive_path = ? WHERE id = ?`,
		len(files), totalSize, archivePath, skillID)
	return err
}

// indexFiles 遍历目录生成文件清单（相对路径、大小、sha256）
func indexFiles(skillID int64, root string) ([]SkillFile, error) {
	var files []SkillFile
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		hash, err := sha256File(path)
		if err != nil {
			return err
		}
		files = append(files, SkillFile{
			SkillID:  skillID,
			FilePath: rel,
			Size:     info.Size(),
			SHA256:   hash,
		})
		return nil
	})
	return files, err
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ---------- 下载 ----------

// downloadSkill GET /api/skills/:id/download
func downloadSkill(c *gin.Context) {
	id := c.Param("id")
	var s Skill
	err := db.QueryRow(`SELECT id, name, archive_path FROM skills WHERE id = ?`, id).
		Scan(&s.ID, &s.Name, &s.ArchivePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
		return
	}

	// 有原始 zip 直接下载；否则现场打包
	zipPath := s.ArchivePath
	if zipPath == "" || !fileExists(zipPath) {
		skillDir := filepath.Join(FilesDir, id)
		zipPath = filepath.Join(ArchiveDir, fmt.Sprintf("%s_download.zip", id))
		if err := zipDirectory(skillDir, zipPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "package failed: " + err.Error()})
			return
		}
	}

	db.Exec(`UPDATE skills SET download_count = download_count + 1 WHERE id = ?`, id)

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, sanitizeFilename(s.Name)))
	c.File(zipPath)
}

// ---------- 删除 ----------

// deleteSkill DELETE /api/skills/:id（仅属主可删）
func deleteSkill(c *gin.Context) {
	id := c.Param("id")
	uid := c.GetInt64("userID")

	var ownerID *int64
	err := db.QueryRow(`SELECT owner_id FROM skills WHERE id = ?`, id).Scan(&ownerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
		return
	}
	if ownerID == nil || *ownerID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅技能属主可删除"})
		return
	}

	db.Exec(`DELETE FROM skill_files WHERE skill_id = ?`, id)
	result, err := db.Exec(`DELETE FROM skills WHERE id = ?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
		return
	}
	// 清理磁盘文件
	os.RemoveAll(filepath.Join(FilesDir, id))
	os.Remove(filepath.Join(ArchiveDir, id+".zip"))
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ---------- 工具 ----------

func getSkillByID(id int64) (*Skill, error) {
	var s Skill
	err := db.QueryRow(`SELECT s.id, s.owner_id, COALESCE(u.username,''), s.name, s.description,
		s.category, s.tags, s.version, s.icon, s.file_count, s.total_size,
		s.download_count, s.view_count, s.rating, s.rating_count, s.created_at, s.updated_at
		FROM skills s LEFT JOIN users u ON s.owner_id = u.id WHERE s.id = ?`, id).
		Scan(&s.ID, &s.OwnerID, &s.OwnerName, &s.Name, &s.Description,
			&s.Category, &s.Tags, &s.Version, &s.Icon, &s.FileCount, &s.TotalSize,
			&s.DownloadCount, &s.ViewCount, &s.Rating, &s.RatingCount, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func nullableInt64(s string) interface{} {
	if s == "" {
		return nil
	}
	var v int64
	fmt.Sscanf(s, "%d", &v)
	return v
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_", " ", "_")
	return replacer.Replace(name)
}
