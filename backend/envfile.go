package main

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func loadLocalEnv() {
	var candidates []string
	if p := strings.TrimSpace(os.Getenv("SKILLHUB_ENV_FILE")); p != "" {
		candidates = append(candidates, p)
	}
	if data := strings.TrimSpace(os.Getenv("SKILLHUB_DATA")); data != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(data), "local.env"))
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "local.env"),
			filepath.Join(dir, "..", "local.env"),
			filepath.Join(dir, "..", "..", "local.env"),
			filepath.Join(dir, ".env"),
			filepath.Join(dir, "..", ".env"),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "local.env"),
			filepath.Join(cwd, "..", "local.env"),
			filepath.Join(cwd, "..", "..", "local.env"),
			filepath.Join(cwd, ".env"),
			filepath.Join(cwd, "..", ".env"),
		)
	}

	seen := map[string]bool{}
	for _, p := range candidates {
		abs, err := filepath.Abs(p)
		if err != nil || seen[abs] {
			continue
		}
		seen[abs] = true
		if loadEnvFile(abs) {
			log.Printf("loaded env file: %s", abs)
			break
		}
	}

	if strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY")) == "" {
		log.Printf("DEEPSEEK_API_KEY missing — put it in local.env next to the project folder")
	} else {
		log.Printf("DEEPSEEK_API_KEY: configured")
	}
}

func loadEnvFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	loaded := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"'`)
		if k == "" || v == "" {
			continue
		}
		if os.Getenv(k) != "" {
			continue
		}
		_ = os.Setenv(k, v)
		loaded = true
	}
	return loaded
}
