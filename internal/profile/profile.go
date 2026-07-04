// Package profile 实现命名配置集（profile）的存储与管理。
// 一个 profile = mixin.yaml + rules.yaml + meta.yaml 的命名集合，
// 存放在 baseDir/profiles/<name>/ 下，原子写入（tmp+rename）。
package profile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Profile 是一个配置集的内存表示。Mixin/Rules 为原始 YAML 字节。
type Profile struct {
	Name  string
	Mixin []byte
	Rules []byte
}

// meta 是 profile 元数据，存 meta.yaml。
type meta struct {
	Name    string `yaml:"name"`
	Created string `yaml:"created"`
}

// Store 管理 baseDir 下的 profile 集合。并发安全。
type Store struct {
	baseDir string
	mu      sync.Mutex
}

// NewStore 创建 Store。baseDir 通常为 ~/.labproxy。
func NewStore(baseDir string) (*Store, error) {
	if baseDir == "" {
		return nil, errors.New("profile: empty base dir")
	}
	if err := os.MkdirAll(filepath.Join(baseDir, "profiles"), 0o755); err != nil {
		return nil, fmt.Errorf("profile: mkdir profiles: %w", err)
	}
	return &Store{baseDir: baseDir}, nil
}

// profilesRoot 返回 profiles 目录绝对路径。
func (s *Store) profilesRoot() string {
	return filepath.Join(s.baseDir, "profiles")
}

// profileDir 返回单个 profile 目录路径。
func (s *Store) profileDir(name string) string {
	return filepath.Join(s.profilesRoot(), name)
}

// validName 校验 profile 名：非空、无路径分隔符、无 .. 、无空格。
// 防止路径遍历。
func validName(name string) error {
	if name == "" {
		return errors.New("profile: empty name")
	}
	if strings.ContainsAny(name, "/\\ ") {
		return errors.New("profile: name must not contain slash, backslash or space")
	}
	if name == "." || name == ".." {
		return errors.New("profile: name must not be . or ..")
	}
	return nil
}

// Create 创建或覆盖一个 profile（原子写入）。
func (s *Store) Create(p Profile) error {
	if err := validName(p.Name); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.profileDir(p.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("profile: mkdir %s: %w", p.Name, err)
	}

	if err := atomicWrite(filepath.Join(dir, "mixin.yaml"), p.Mixin); err != nil {
		return fmt.Errorf("profile: write mixin: %w", err)
	}
	if err := atomicWrite(filepath.Join(dir, "rules.yaml"), p.Rules); err != nil {
		return fmt.Errorf("profile: write rules: %w", err)
	}

	m := meta{Name: p.Name, Created: time.Now().Format(time.RFC3339)}
	metaBytes := []byte(fmt.Sprintf("name: %s\ncreated: %s\n", m.Name, m.Created))
	if err := atomicWrite(filepath.Join(dir, "meta.yaml"), metaBytes); err != nil {
		return fmt.Errorf("profile: write meta: %w", err)
	}
	return nil
}

// atomicWrite 原子写入文件：先写唯一 tmp 再 rename，避免半写状态与多进程 tmp 冲突。
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".profile-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	tmp.Close()
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// Load 读取一个 profile。
func (s *Store) Load(name string) (Profile, error) {
	if err := validName(name); err != nil {
		return Profile{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.profileDir(name)
	if _, err := os.Stat(dir); err != nil {
		return Profile{}, fmt.Errorf("profile: %s not found", name)
	}

	p := Profile{Name: name}
	mixin, err := os.ReadFile(filepath.Join(dir, "mixin.yaml"))
	if err != nil {
		return Profile{}, fmt.Errorf("profile: read mixin: %w", err)
	}
	p.Mixin = mixin
	rules, err := os.ReadFile(filepath.Join(dir, "rules.yaml"))
	if err != nil {
		return Profile{}, fmt.Errorf("profile: read rules: %w", err)
	}
	p.Rules = rules
	return p, nil
}

// List 列出所有 profile 名。
func (s *Store) List() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.profilesRoot())
	if err != nil {
		return nil, fmt.Errorf("profile: list: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// Delete 删除一个 profile。
func (s *Store) Delete(name string) error {
	if err := validName(name); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.profileDir(name)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("profile: %s not found", name)
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("profile: delete %s: %w", name, err)
	}
	return nil
}
