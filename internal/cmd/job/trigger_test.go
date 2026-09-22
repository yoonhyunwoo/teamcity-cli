package job_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobTriggerList(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "job", "trigger", "list", testJob)
	assert.Contains(t, out, "TRIGGER_1")
	assert.Contains(t, out, "VCS")
	assert.Contains(t, out, "TRIGGER_2")
	assert.Contains(t, out, "cronExpression=0 0 2 * *?")
}

func TestJobTriggerListJSON(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "job", "trigger", "list", testJob, "--json")
	assert.Contains(t, out, `"vcsTrigger"`)
	assert.Contains(t, out, `"scheduleTrigger"`)
}

func TestJobTriggerAddVCS(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	var gotPath, gotBody string
	ts.Handle("POST /app/rest/buildTypes/id:", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		gotPath = r.URL.Path
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"TRIGGER_1","type":"vcsTrigger"}`))
	})

	out := cmdtest.CaptureOutput(t, ts.Factory, "job", "trigger", "add", testJob, "--type", "vcs")
	assert.Equal(t, "/app/rest/buildTypes/id:"+testJob+"/triggers", gotPath)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(gotBody), &payload))
	assert.Equal(t, "vcsTrigger", payload["type"])
	assert.NotContains(t, string(gotBody), "perCheckinTriggering")
	assert.Contains(t, out, "Added VCS trigger (id: TRIGGER_1)")
}

func TestJobTriggerAddVCSPerCheckin(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	var gotBody string
	ts.Handle("POST /app/rest/buildTypes/id:", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"TRIGGER_1","type":"vcsTrigger"}`))
	})

	cmdtest.CaptureOutput(t, ts.Factory, "job", "trigger", "add", testJob, "--type", "vcs", "--per-checkin")
	assert.Contains(t, gotBody, `"name":"perCheckinTriggering","value":"true"`)
}

func TestJobTriggerAddSchedule(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	var gotBody string
	ts.Handle("POST /app/rest/buildTypes/id:", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"TRIGGER_2","type":"scheduleTrigger"}`))
	})

	out := cmdtest.CaptureOutput(t, ts.Factory, "job", "trigger", "add", testJob, "--type", "schedule", "--cron", "0 0 2 * *?")
	assert.Contains(t, gotBody, `"name":"cronExpression","value":"0 0 2 * *?"`)
	assert.Contains(t, out, "Added Schedule trigger")
}

func TestJobTriggerAddScheduleRequiresCron(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "--cron", "job", "trigger", "add", testJob, "--type", "schedule")
}

func TestJobTriggerAddFinishBuild(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	var gotBody string
	ts.Handle("POST /app/rest/buildTypes/id:", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"TRIGGER_3","type":"finishBuildTrigger"}`))
	})

	out := cmdtest.CaptureOutput(t, ts.Factory, "job", "trigger", "add", testJob, "--type", "finish-build", "--depends-on", "OtherBuild")
	assert.Contains(t, gotBody, `"name":"dependsOn","value":"OtherBuild"`)
	assert.Contains(t, out, "Added Finish Build trigger")
}

func TestJobTriggerAddFinishBuildRequiresDependsOn(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "--depends-on", "job", "trigger", "add", testJob, "--type", "finish-build")
}

func TestJobTriggerAddRejectsUnknownType(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "unknown trigger type", "job", "trigger", "add", testJob, "--type", "maven")
}

func TestJobTriggerAddRejectsMismatchedFlag(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "--cron only applies",
		"job", "trigger", "add", testJob, "--type", "vcs", "--cron", "0 0 2 * *?")
}

func TestJobTriggerDelete(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	var gotMethod, gotPath string
	ts.Handle("DELETE /app/rest/buildTypes/id:", func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})

	out := cmdtest.CaptureOutput(t, ts.Factory, "job", "trigger", "delete", testJob, "TRIGGER_1")
	assert.Equal(t, http.MethodDelete, gotMethod)
	assert.Equal(t, "/app/rest/buildTypes/id:"+testJob+"/triggers/TRIGGER_1", gotPath)
	assert.Contains(t, out, "Deleted trigger TRIGGER_1")
}
