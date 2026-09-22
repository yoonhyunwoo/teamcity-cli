package job

import (
	"fmt"
	"strconv"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

func newJobTriggerCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trigger",
		Short: "Manage job triggers",
		Long: `List, add, and delete triggers on a job (build configuration).

A trigger starts builds automatically: a VCS trigger fires on new
check-ins, a schedule trigger on a cron expression, and a finish-build
trigger when another build configuration finishes.`,
		Args: cobra.NoArgs,
		RunE: cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newJobTriggerListCmd(f))
	cmd.AddCommand(newJobTriggerAddCmd(f))
	cmd.AddCommand(newJobTriggerDeleteCmd(f))

	return cmd
}

type jobTriggerListOptions struct {
	cmdutil.ListOptions
}

func newJobTriggerListCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &jobTriggerListOptions{}

	cmd := &cobra.Command{
		Use:               "list [job-id]",
		Short:             "List job triggers",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cmdutil.CompleteOwnerID(completion.LinkedJobs()),
		Example: `  teamcity job trigger list MyBuild
  teamcity job trigger list                 # uses linked job (see 'teamcity link')
  teamcity job trigger list MyBuild --json
  teamcity job trigger list MyBuild --plain`,
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID, _, err := cmdutil.ResolveOwnerID("job", args, 0, f.ResolveDefaultJob)
			if err != nil {
				return err
			}
			return runJobTriggerList(f, jobID, opts)
		},
	}

	opts.AddFlags(cmd, false)

	return cmd
}

func runJobTriggerList(f *cmdutil.Factory, jobID string, opts *jobTriggerListOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	triggers, err := client.GetBuildTriggers(jobID)
	if err != nil {
		return err
	}

	if opts.JSON {
		return f.Printer.PrintJSON(triggers)
	}

	p := f.Printer
	if triggers.Count == 0 {
		p.Empty("No triggers found", "Add one with 'teamcity job trigger add "+jobID+" --type vcs'")
		return nil
	}

	headers := []string{"#", "ID", "TYPE", "DETAILS"}
	var rows [][]string
	for i, t := range triggers.Trigger {
		rows = append(rows, []string{
			strconv.Itoa(i + 1),
			t.ID,
			triggerTypeName(t.Type),
			triggerDetail(t),
		})
	}

	if opts.Plain {
		p.PrintPlainTable(headers, rows, opts.NoHeader)
	} else {
		output.AutoSizeColumns(headers, rows, 2, 3)
		p.PrintTable(headers, rows)
	}
	return nil
}

var triggerTypeNames = map[string]string{
	"vcsTrigger":         "VCS",
	"scheduleTrigger":    "Schedule",
	"finishBuildTrigger": "Finish Build",
}

func triggerTypeName(t string) string {
	if name, ok := triggerTypeNames[t]; ok {
		return name
	}
	return t
}

// triggerDetail surfaces the one property that identifies what the trigger does.
func triggerDetail(t api.BuildTrigger) string {
	for _, p := range t.Properties.Property {
		switch p.Name {
		case "cronExpression", "dependsOn", "perCheckinTriggering":
			return fmt.Sprintf("%s=%s", p.Name, p.Value)
		}
	}
	return ""
}

type jobTriggerAddOptions struct {
	triggerType string
	cron        string
	dependsOn   string
	perCheckin  bool
	json        bool
}

func newJobTriggerAddCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &jobTriggerAddOptions{}

	cmd := &cobra.Command{
		Use:               "add [job-id] --type vcs|schedule|finish-build",
		Short:             "Add a trigger to a job",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: cmdutil.CompleteOwnerID(completion.LinkedJobs()),
		Example: `  teamcity job trigger add MyBuild --type vcs
  teamcity job trigger add MyBuild --type vcs --per-checkin
  teamcity job trigger add MyBuild --type schedule --cron "0 0 2 * *?"
  teamcity job trigger add MyBuild --type finish-build --depends-on OtherBuild`,
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID, _, err := cmdutil.ResolveOwnerID("job", args, 0, f.ResolveDefaultJob)
			if err != nil {
				return err
			}
			return runJobTriggerAdd(f, jobID, opts)
		},
	}

	cmd.Flags().StringVar(&opts.triggerType, "type", "", "Trigger type: vcs|schedule|finish-build")
	cmd.Flags().StringVar(&opts.cron, "cron", "", "Cron expression (schedule triggers only)")
	cmd.Flags().StringVar(&opts.dependsOn, "depends-on", "", "Build type ID to wait for (finish-build triggers only)")
	cmd.Flags().BoolVar(&opts.perCheckin, "per-checkin", false, "Run a separate build per check-in (vcs triggers only)")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output the created trigger as JSON")
	_ = cmd.MarkFlagRequired("type")

	return cmd
}

func runJobTriggerAdd(f *cmdutil.Factory, jobID string, opts *jobTriggerAddOptions) error {
	if opts.cron != "" && opts.triggerType != "schedule" {
		return api.Validation("--cron only applies to --type schedule", "Drop --cron or use --type schedule")
	}
	if opts.dependsOn != "" && opts.triggerType != "finish-build" {
		return api.Validation("--depends-on only applies to --type finish-build", "Drop --depends-on or use --type finish-build")
	}
	if opts.perCheckin && opts.triggerType != "vcs" {
		return api.Validation("--per-checkin only applies to --type vcs", "Drop --per-checkin or use --type vcs")
	}

	var trigger api.BuildTrigger
	switch opts.triggerType {
	case "vcs":
		trigger = api.BuildTrigger{Type: "vcsTrigger"}
		if opts.perCheckin {
			trigger.Properties = api.PropertyList{Property: []api.Property{
				{Name: "perCheckinTriggering", Value: "true"},
			}}
		}
	case "schedule":
		if opts.cron == "" {
			return api.RequiredFlag("cron")
		}
		trigger = api.BuildTrigger{Type: "scheduleTrigger", Properties: api.PropertyList{Property: []api.Property{
			{Name: "schedule", Value: "cron"},
			{Name: "cronExpression", Value: opts.cron},
		}}}
	case "finish-build":
		if opts.dependsOn == "" {
			return api.RequiredFlag("depends-on")
		}
		trigger = api.BuildTrigger{Type: "finishBuildTrigger", Properties: api.PropertyList{Property: []api.Property{
			{Name: "dependsOn", Value: opts.dependsOn},
		}}}
	default:
		return api.Validation(fmt.Sprintf("unknown trigger type %q", opts.triggerType), "Use vcs, schedule, or finish-build")
	}

	client, err := f.Client()
	if err != nil {
		return err
	}

	created, err := client.CreateBuildTrigger(jobID, trigger)
	if err != nil {
		return fmt.Errorf("failed to add trigger: %w", err)
	}

	if opts.json {
		return f.Printer.PrintJSON(created)
	}

	f.Printer.Success("Added %s trigger (id: %s) to job %s", triggerTypeName(created.Type), created.ID, jobID)
	return nil
}

type jobTriggerDeleteOptions struct{}

func newJobTriggerDeleteCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "delete [job-id] <trigger-id>",
		Short:             "Delete a job trigger",
		Aliases:           []string{"remove", "rm"},
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: cmdutil.CompleteOwnerID(completion.LinkedJobs()),
		Example: `  teamcity job trigger delete MyBuild TRIGGER_1
  teamcity job trigger delete TRIGGER_1      # uses linked job`,
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID, rest, err := cmdutil.ResolveOwnerID("job", args, 1, f.ResolveDefaultJob)
			if err != nil {
				return err
			}
			return runJobTriggerDelete(f, jobID, rest[0])
		},
	}

	return cmd
}

func runJobTriggerDelete(f *cmdutil.Factory, jobID, triggerID string) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	if err := client.DeleteBuildTrigger(jobID, triggerID); err != nil {
		return fmt.Errorf("failed to delete trigger: %w", err)
	}

	f.Printer.Success("Deleted trigger %s", triggerID)
	return nil
}
