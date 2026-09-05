package main

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	stagingDir := flag.String("staging", "", "Staging directory containing DEBIAN, usr, etc...")
	outputDeb := flag.String("output", "", "Output .deb file path")
	flag.Parse()

	if *stagingDir == "" || *outputDeb == "" {
		log.Fatalf("Usage: debpack -staging <dir> -output <file.deb>")
	}

	// 1. 生成 control.tar.gz
	controlGz, err := createTarGz(filepath.Join(*stagingDir, "DEBIAN"), "")
	if err != nil {
		log.Fatalf("Failed to create control.tar.gz: %v", err)
	}

	// 2. 生成 data.tar.gz (排除 DEBIAN 目录)
	dataGz, err := createTarGz(*stagingDir, "DEBIAN")
	if err != nil {
		log.Fatalf("Failed to create data.tar.gz: %v", err)
	}

	// 3. 组装为符合 Debian 规范的 ar 格式文件
	debFile, err := os.Create(*outputDeb)
	if err != nil {
		log.Fatalf("Failed to create deb file: %v", err)
	}
	defer debFile.Close()

	// 写入 ar magic header
	if _, err := debFile.WriteString("!<arch>\n"); err != nil {
		log.Fatalf("Failed to write ar magic: %v", err)
	}

	// 写入 debian-binary 成员
	debianBinaryContent := []byte("2.0\n")
	writeArEntry(debFile, "debian-binary", debianBinaryContent)

	// 写入 control.tar.gz 成员
	writeArEntry(debFile, "control.tar.gz", controlGz)

	// 写入 data.tar.gz 成员
	writeArEntry(debFile, "data.tar.gz", dataGz)

	fmt.Printf("[SUCCESS] Successfully packaged: %s\n", *outputDeb)
}

func writeArEntry(w io.Writer, name string, data []byte) {
	// AR Header: 60 bytes
	// 0..16: File name
	// 16..28: Timestamp
	// 28..34: Owner ID
	// 34..40: Group ID
	// 40..48: File mode
	// 48..58: File size
	// 58..60: Trailing magic (\x60\n)
	header := fmt.Sprintf("%-16s%-12d%-6d%-6d%-8o%-10d`\n",
		name,
		time.Now().Unix(),
		0,
		0,
		0100644,
		len(data),
	)
	w.Write([]byte(header))
	w.Write(data)
	if len(data)%2 != 0 {
		// AR 规范：奇数长度需追加一个换行 '\n' 作为 padding
		w.Write([]byte("\n"))
	}
}

func createTarGz(sourceDir string, excludeDir string) ([]byte, error) {
	var buf strings.Builder
	_ = buf
	tempFile, err := os.CreateTemp("", "debpack-*.tar.gz")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	gw := gzip.NewWriter(tempFile)
	tw := tar.NewWriter(gw)

	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		// 排除指定目录 (如 DEBIAN)
		if excludeDir != "" {
			parts := strings.Split(relPath, string(filepath.Separator))
			if parts[0] == excludeDir {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// 转为标准 Linux 正斜杠路径，并带前导 ./
		tarPath := "./" + filepath.ToSlash(relPath)

		var link string
		if info.Mode()&os.ModeSymlink != 0 {
			link, _ = os.Readlink(path)
		}

		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}

		header.Name = tarPath
		header.Uname = "root"
		header.Gname = "root"
		header.Uid = 0
		header.Gid = 0

		// 保持 Linux 权限，二进制和维护脚本强制为 0755
		if strings.HasSuffix(tarPath, "/noder") || strings.Contains(sourceDir, "DEBIAN") {
			if strings.HasSuffix(tarPath, "postinst") || strings.HasSuffix(tarPath, "prerm") || strings.HasSuffix(tarPath, "postrm") || strings.HasSuffix(tarPath, "/noder") {
				header.Mode = 0755
			} else {
				header.Mode = 0644
			}
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}

	return os.ReadFile(tempFile.Name())
}
