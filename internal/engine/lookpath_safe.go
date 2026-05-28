package engine

import (
  "fmt"
  "os"
  "path/filepath"
  "strings"
  "sync"
)

var (
  pathCache   = make(map[string]string)
  pathCacheMu sync.RWMutex
)

// lookPathSafe 在不依赖 faccessat2 的情况下查找可执行文件。
// Go 标准库的 exec.LookPath 使用 faccessat2 系统调用，
// 该调用在 Android 内核上不被支持，会导致 SIGSYS 崩溃。
// 通过进程内缓存避免同一命令的重复 PATH 遍历。
func lookPathSafe(name string) (string, error) {
  if strings.Contains(name, "/") {
    return filepath.Abs(name)
  }

  pathCacheMu.RLock()
  cached, ok := pathCache[name]
  pathCacheMu.RUnlock()
  if ok {
    return cached, nil
  }

  pathEnv := os.Getenv("PATH")
  for _, dir := range filepath.SplitList(pathEnv) {
    if dir == "" {
      dir = "."
    }
    fullPath := filepath.Join(dir, name)
    info, err := os.Stat(fullPath)
    if err != nil {
      continue
    }
    if info.IsDir() {
      continue
    }
    if info.Mode()&0111 != 0 {
      pathCacheMu.Lock()
      pathCache[name] = fullPath
      pathCacheMu.Unlock()
      return fullPath, nil
    }
  }
  return "", fmt.Errorf("executable file not found in $PATH: %s", name)
}
