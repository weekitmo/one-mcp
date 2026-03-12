package handler

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"one-mcp/backend/common"
	"one-mcp/backend/common/i18n"
	"one-mcp/backend/model"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	skillsStorageDir    = filepath.Join("data", "skills")
	gitIgnoreFilePath   = ".gitignore"
	gitIgnoreSkillEntry = "data/skills/"
)

var skillConfigureAgents = []string{"codex", "claude-code", "gemini-cli", "antigravity"}

var runSkillsAddCommand = executeSkillsAddCommand

// ConfigureGroupSkill exports and configures a group skill package in one click.
// POST /api/groups/:id/configure-skill
func ConfigureGroupSkill(c *gin.Context) {
	lang := c.GetString("lang")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.RespErrorStr(c, http.StatusBadRequest, i18n.Translate("invalid_param", lang))
		return
	}

	userID := c.GetInt64("user_id")
	group, err := model.GetMCPServiceGroupByID(id, userID)
	if err != nil {
		common.RespError(c, http.StatusNotFound, "group not found", err)
		return
	}
	common.SysLog(fmt.Sprintf("Configure group skill started | group_id=%d | group_name=%s | user_id=%d", group.ID, group.Name, userID))

	user, err := model.GetUserById(userID, false)
	if err != nil {
		common.RespError(c, http.StatusInternalServerError, "failed to get user", err)
		return
	}

	serverAddress := common.OptionMap["ServerAddress"]
	if serverAddress == "" {
		serverAddress = c.Request.Host
		scheme := "https"
		if c.Request.TLS == nil && !strings.HasPrefix(c.Request.Header.Get("X-Forwarded-Proto"), "https") {
			scheme = "http"
		}
		serverAddress = scheme + "://" + serverAddress
	}

	zipBuffer, err := buildSkillZip(c.Request.Context(), group, user, serverAddress)
	if err != nil {
		common.RespError(c, http.StatusInternalServerError, "failed to generate skill zip", err)
		return
	}

	skillName := "one-mcp-" + normalizeSkillName(group.Name)
	if err := configureSkillZip(c.Request.Context(), skillName, zipBuffer.Bytes()); err != nil {
		common.SysError(fmt.Sprintf("Configure group skill failed | group_id=%d | group_name=%s | user_id=%d | error=%v", group.ID, group.Name, userID, err))
		common.RespError(c, http.StatusInternalServerError, "failed to configure skill", err)
		return
	}

	common.SysLog(fmt.Sprintf("Configure group skill completed | group_id=%d | group_name=%s | user_id=%d", group.ID, group.Name, userID))
	common.RespSuccessStr(c, "skill configured")
}

func configureSkillZip(ctx context.Context, skillName string, zipData []byte) error {
	startTime := time.Now()
	common.SysLog(fmt.Sprintf("Configure group skill step | skill=%s | step=start", skillName))

	if err := os.MkdirAll(skillsStorageDir, 0o755); err != nil {
		common.SysError(fmt.Sprintf("Configure group skill step failed | skill=%s | step=mkdir-skills-dir | error=%v", skillName, err))
		return fmt.Errorf("create skills dir: %w", err)
	}
	if err := ensureGitignoreEntry(gitIgnoreFilePath, gitIgnoreSkillEntry); err != nil {
		common.SysError(fmt.Sprintf("Configure group skill step failed | skill=%s | step=ensure-gitignore | error=%v", skillName, err))
		return fmt.Errorf("update .gitignore: %w", err)
	}

	zipPath, err := filepath.Abs(filepath.Join(skillsStorageDir, skillName+".zip"))
	if err != nil {
		common.SysError(fmt.Sprintf("Configure group skill step failed | skill=%s | step=abs-zip-path | error=%v", skillName, err))
		return fmt.Errorf("resolve zip path: %w", err)
	}
	skillPath, err := filepath.Abs(filepath.Join(skillsStorageDir, skillName))
	if err != nil {
		common.SysError(fmt.Sprintf("Configure group skill step failed | skill=%s | step=abs-skill-path | error=%v", skillName, err))
		return fmt.Errorf("resolve skill path: %w", err)
	}
	common.SysLog(fmt.Sprintf("Configure group skill step | skill=%s | step=paths-ready | zip=%s | dir=%s", skillName, zipPath, skillPath))

	if err := os.WriteFile(zipPath, zipData, 0o644); err != nil {
		common.SysError(fmt.Sprintf("Configure group skill step failed | skill=%s | step=write-zip | error=%v", skillName, err))
		return fmt.Errorf("write skill zip: %w", err)
	}
	common.SysLog(fmt.Sprintf("Configure group skill step | skill=%s | step=zip-written | bytes=%d", skillName, len(zipData)))

	cleanupArtifacts := true
	defer func() {
		if cleanupArtifacts {
			_ = os.Remove(zipPath)
			_ = os.RemoveAll(skillPath)
			return
		}
		common.SysLog(fmt.Sprintf("Configure group skill artifacts retained for debugging | skill=%s | zip=%s | dir=%s", skillName, zipPath, skillPath))
	}()

	if err := os.RemoveAll(skillPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		common.SysError(fmt.Sprintf("Configure group skill step failed | skill=%s | step=cleanup-existing-dir | error=%v", skillName, err))
		return fmt.Errorf("prepare skill dir: %w", err)
	}
	if err := unzipToDir(zipPath, skillPath); err != nil {
		common.SysError(fmt.Sprintf("Configure group skill step failed | skill=%s | step=unzip | error=%v", skillName, err))
		return fmt.Errorf("extract skill zip: %w", err)
	}
	common.SysLog(fmt.Sprintf("Configure group skill step | skill=%s | step=unzipped", skillName))

	commandCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	common.SysLog(fmt.Sprintf("Configure group skill step | skill=%s | step=run-command | path=%s | command=npx -y skills add -g <path> -a codex -a claude-code -a gemini-cli -a antigravity -y", skillName, skillPath))

	output, err := runSkillsAddCommand(commandCtx, skillPath)
	if err != nil {
		cleanupArtifacts = false
		trimmedOutput := strings.TrimSpace(string(output))
		if trimmedOutput != "" {
			common.SysError(fmt.Sprintf("Configure group skill step failed | skill=%s | step=run-command | error=%v | output=%s", skillName, err, trimmedOutput))
			return fmt.Errorf("run skills add command: %w, output: %s, artifacts retained: zip=%s, dir=%s", err, trimmedOutput, zipPath, skillPath)
		}
		common.SysError(fmt.Sprintf("Configure group skill step failed | skill=%s | step=run-command | error=%v", skillName, err))
		return fmt.Errorf("run skills add command: %w, artifacts retained: zip=%s, dir=%s", err, zipPath, skillPath)
	}

	common.SysLog(fmt.Sprintf("Configure group skill completed | skill=%s | duration_ms=%d", skillName, time.Since(startTime).Milliseconds()))
	return nil
}

func ensureGitignoreEntry(gitignorePath string, entry string) error {
	trimmedEntry := strings.TrimSpace(entry)
	if trimmedEntry == "" {
		return nil
	}

	dir := filepath.Dir(gitignorePath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.WriteFile(gitignorePath, []byte(trimmedEntry+"\n"), 0o644)
		}
		return err
	}

	normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
	for _, line := range strings.Split(normalized, "\n") {
		if strings.TrimSpace(line) == trimmedEntry {
			return nil
		}
	}

	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	if len(content) > 0 && content[len(content)-1] != '\n' {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	_, err = f.WriteString(trimmedEntry + "\n")
	return err
}

func unzipToDir(zipPath string, targetDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	cleanTargetDir := filepath.Clean(targetDir)
	targetPrefix := cleanTargetDir + string(os.PathSeparator)

	for _, file := range reader.File {
		cleanName := filepath.Clean(file.Name)
		if cleanName == "." {
			continue
		}

		targetPath := filepath.Join(cleanTargetDir, cleanName)
		if targetPath != cleanTargetDir && !strings.HasPrefix(targetPath, targetPrefix) {
			return fmt.Errorf("invalid zip path: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		in, err := file.Open()
		if err != nil {
			return err
		}

		out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			in.Close()
			return err
		}

		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		inCloseErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if inCloseErr != nil {
			return inCloseErr
		}
	}

	return nil
}

func executeSkillsAddCommand(ctx context.Context, skillPath string) ([]byte, error) {
	args := []string{"-y", "skills", "add", "-g", skillPath}
	for _, agent := range skillConfigureAgents {
		args = append(args, "-a", agent)
	}
	args = append(args, "-y")

	cmd := exec.CommandContext(ctx, "npx", args...)
	return cmd.CombinedOutput()
}
