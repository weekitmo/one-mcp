package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"one-mcp/backend/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestConfigureGroupSkillSuccess(t *testing.T) {
	teardown := setupGroupTestDB(t)
	defer teardown()

	gin.SetMode(gin.TestMode)

	user := &model.User{
		Username:    "skill-config-user",
		Password:    "password123",
		DisplayName: "Skill Config User",
		Email:       "skill-config-user@example.com",
		Token:       "test-skill-token",
	}
	assert.NoError(t, user.Insert())

	group := &model.MCPServiceGroup{
		UserID:      user.ID,
		Name:        "group_one",
		DisplayName: "Group One",
		Description: "test group",
		Enabled:     true,
	}
	group.SetServiceIDs([]int64{})
	assert.NoError(t, group.Insert())

	tmpDir := t.TempDir()
	originalSkillsStorageDir := skillsStorageDir
	originalGitIgnoreFilePath := gitIgnoreFilePath
	originalRunSkillsAddCommand := runSkillsAddCommand

	skillsStorageDir = filepath.Join(tmpDir, "data", "skills")
	gitIgnoreFilePath = filepath.Join(tmpDir, ".gitignore")

	var configuredSkillPath string
	runSkillsAddCommand = func(_ context.Context, skillPath string) ([]byte, error) {
		configuredSkillPath = skillPath
		_, err := os.Stat(filepath.Join(skillPath, "SKILL.md"))
		assert.NoError(t, err)
		return []byte("ok"), nil
	}

	defer func() {
		skillsStorageDir = originalSkillsStorageDir
		gitIgnoreFilePath = originalGitIgnoreFilePath
		runSkillsAddCommand = originalRunSkillsAddCommand
	}()

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("/api/groups/%d/configure-skill", group.ID), nil)
	assert.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", group.ID)}}
	ctx.Set("user_id", user.ID)
	ctx.Set("lang", "en")

	ConfigureGroupSkill(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	resp := decodeAPIResponse(t, recorder)
	assert.True(t, resp.Success)
	assert.Equal(t, filepath.Join(skillsStorageDir, "one-mcp-group-one"), configuredSkillPath)

	_, err = os.Stat(filepath.Join(skillsStorageDir, "one-mcp-group-one.zip"))
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(configuredSkillPath)
	assert.True(t, os.IsNotExist(err))

	content, err := os.ReadFile(gitIgnoreFilePath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), gitIgnoreSkillEntry)
}

func TestConfigureGroupSkillFailureIncludesStepMessage(t *testing.T) {
	teardown := setupGroupTestDB(t)
	defer teardown()

	gin.SetMode(gin.TestMode)

	user := &model.User{
		Username:    "skill-config-user-failure",
		Password:    "password123",
		DisplayName: "Skill Config User Failure",
		Email:       "skill-config-user-failure@example.com",
		Token:       "test-skill-token-failure",
	}
	assert.NoError(t, user.Insert())

	group := &model.MCPServiceGroup{
		UserID:      user.ID,
		Name:        "group_two",
		DisplayName: "Group Two",
		Description: "test group",
		Enabled:     true,
	}
	group.SetServiceIDs([]int64{})
	assert.NoError(t, group.Insert())

	tmpDir := t.TempDir()
	originalSkillsStorageDir := skillsStorageDir
	originalGitIgnoreFilePath := gitIgnoreFilePath
	originalRunSkillsAddCommand := runSkillsAddCommand

	skillsStorageDir = filepath.Join(tmpDir, "data", "skills")
	gitIgnoreFilePath = filepath.Join(tmpDir, ".gitignore")
	runSkillsAddCommand = func(_ context.Context, _ string) ([]byte, error) {
		return []byte("mock command stderr"), errors.New("exit status 1")
	}

	defer func() {
		skillsStorageDir = originalSkillsStorageDir
		gitIgnoreFilePath = originalGitIgnoreFilePath
		runSkillsAddCommand = originalRunSkillsAddCommand
	}()

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("/api/groups/%d/configure-skill", group.ID), nil)
	assert.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", group.ID)}}
	ctx.Set("user_id", user.ID)
	ctx.Set("lang", "en")

	ConfigureGroupSkill(ctx)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	resp := decodeAPIResponse(t, recorder)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "failed to configure skill: run skills add command")
	assert.Contains(t, resp.Message, "mock command stderr")
	assert.Contains(t, resp.Message, "artifacts retained")

	_, err = os.Stat(filepath.Join(skillsStorageDir, "one-mcp-group-two.zip"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(skillsStorageDir, "one-mcp-group-two"))
	assert.NoError(t, err)
}
