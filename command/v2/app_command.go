package v2

import (
	"fmt"
	"os"
	"strings"

	"code.cloudfoundry.org/cli/actor/sharedaction"
	"code.cloudfoundry.org/cli/actor/v2action"
	oldCmd "code.cloudfoundry.org/cli/cf/cmd"
	"code.cloudfoundry.org/cli/command"
	"code.cloudfoundry.org/cli/command/flag"
	"code.cloudfoundry.org/cli/command/v2/shared"
	"github.com/cloudfoundry/bytefmt"
)

//go:generate counterfeiter . AppActor

type AppActor interface {
	GetApplicationByNameAndSpace(name string, spaceGUID string) (v2action.Application, v2action.Warnings, error)
	GetApplicationSummaryByNameAndSpace(name string, spaceGUID string) (v2action.ApplicationSummary, v2action.Warnings, error)
}

type AppCommand struct {
	RequiredArgs    flag.AppName `positional-args:"yes"`
	GUID            bool         `long:"guid" description:"Retrieve and display the given app's guid.  All other health and status output for the app is suppressed."`
	usage           interface{}  `usage:"CF_NAME app APP_NAME"`
	relatedCommands interface{}  `related_commands:"apps, events, logs, map-route, unmap-route, push"`

	UI          command.UI
	Config      command.Config
	SharedActor command.SharedActor
	Actor       AppActor
}

func (cmd *AppCommand) Setup(config command.Config, ui command.UI) error {
	cmd.UI = ui
	cmd.Config = config
	cmd.SharedActor = sharedaction.NewActor()

	ccClient, uaaClient, err := shared.NewClients(config, ui)
	if err != nil {
		return err
	}
	cmd.Actor = v2action.NewActor(ccClient, uaaClient)

	return nil
}

func (cmd AppCommand) Execute(args []string) error {
	if cmd.Config.Experimental() == false {
		oldCmd.Main(os.Getenv("CF_TRACE"), os.Args)
		return nil
	}

	cmd.UI.DisplayText(command.ExperimentalWarning)
	cmd.UI.DisplayNewline()

	err := cmd.SharedActor.CheckTarget(cmd.Config, true, true)
	if err != nil {
		return shared.HandleError(err)
	}

	user, err := cmd.Config.CurrentUser()
	if err != nil {
		return shared.HandleError(err)
	}

	if cmd.GUID {
		return cmd.displayAppGUID()
	}

	cmd.UI.DisplayTextWithFlavor(
		"Showing health and status for app {{.AppName}} in org {{.OrgName}} / space {{.SpaceName}} as {{.Username}}...",
		map[string]interface{}{
			"AppName":   cmd.RequiredArgs.AppName,
			"OrgName":   cmd.Config.TargetedOrganization().Name,
			"SpaceName": cmd.Config.TargetedSpace().Name,
			"Username":  user.Name,
		})
	cmd.UI.DisplayNewline()

	appSummary, warnings, err := cmd.Actor.GetApplicationSummaryByNameAndSpace(cmd.RequiredArgs.AppName, cmd.Config.TargetedSpace().GUID)
	cmd.UI.DisplayWarnings(warnings)
	if err != nil {
		return shared.HandleError(err)
	}

	cmd.displayAppSummary(appSummary)

	if len(appSummary.RunningInstances) == 0 {
		cmd.UI.DisplayText("There are no running instances of this app.")
	} else {
		cmd.displayAppInstances(appSummary.RunningInstances)
	}

	return nil
}

func (cmd AppCommand) displayAppGUID() error {
	app, warnings, err := cmd.Actor.GetApplicationByNameAndSpace(cmd.RequiredArgs.AppName, cmd.Config.TargetedSpace().GUID)
	cmd.UI.DisplayWarnings(warnings)
	if err != nil {
		return shared.HandleError(err)
	}

	cmd.UI.DisplayText(app.GUID)
	return nil
}

func (cmd AppCommand) displayAppSummary(appSummary v2action.ApplicationSummary) {
	instances := fmt.Sprintf("%d/%d", len(appSummary.RunningInstances), appSummary.Instances)

	usage := cmd.UI.TranslateText(
		"{{.MemorySize}} x {{.NumInstances}} instances",
		map[string]interface{}{
			"MemorySize":   bytefmt.ByteSize(uint64(appSummary.Memory) * bytefmt.MEGABYTE),
			"NumInstances": appSummary.Instances,
		})

	formattedRoutes := []string{}
	for _, route := range appSummary.Routes {
		formattedRoutes = append(formattedRoutes, route.String())
	}
	routes := strings.Join(formattedRoutes, ", ")

	table := [][]string{
		{cmd.UI.TranslateText("Name:"), appSummary.Name},
		{cmd.UI.TranslateText("Requested state:"), strings.ToLower(string(appSummary.State))},
		{cmd.UI.TranslateText("Instances:"), instances},
		{cmd.UI.TranslateText("Usage:"), usage},
		{cmd.UI.TranslateText("Routes:"), routes},
		{cmd.UI.TranslateText("Last uploaded:"), cmd.UI.UserFriendlyDate(appSummary.PackageUpdatedAt)},
		{cmd.UI.TranslateText("Stack:"), appSummary.Stack.Name},
		{cmd.UI.TranslateText("Buildpack:"), appSummary.Application.CalculatedBuildpack()},
	}

	cmd.UI.DisplayTable("", table, 3)
	cmd.UI.DisplayNewline()
}

func (cmd AppCommand) displayAppInstances(instances []v2action.ApplicationInstance) {
	table := [][]string{
		{"", "State", "Since", "CPU", "Memory", "Disk", "Details"},
	}

	for _, instance := range instances {
		table = append(
			table,
			[]string{
				fmt.Sprintf("#%d", instance.ID),
				cmd.UI.TranslateText(strings.ToLower(string(instance.State))),
				cmd.UI.UserFriendlyDate(instance.TimeSinceCreation()),
				fmt.Sprintf("%.1f%%", instance.CPU*100),
				fmt.Sprintf("%s of %s", bytefmt.ByteSize(uint64(instance.Memory)), bytefmt.ByteSize(uint64(instance.MemoryQuota))),
				fmt.Sprintf("%s of %s", bytefmt.ByteSize(uint64(instance.Disk)), bytefmt.ByteSize(uint64(instance.DiskQuota))),
				instance.Details,
			})
	}

	cmd.UI.DisplayTable("", table, 3)
	return
}
