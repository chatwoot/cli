package cmd

import (
	"fmt"
	"strconv"
)

type AgentCmd struct {
	List AgentListCmd `cmd:"" default:"1" help:"List agents."`
}

type AgentListCmd struct{}

func (c *AgentListCmd) Run(app *App) error {
	agents, err := app.Client.Agents().List()
	if err != nil {
		return err
	}

	if app.Printer.Format == "json" && !app.Printer.Quiet {
		app.Printer.PrintJSON(agents)
		return nil
	}

	if len(agents) == 0 {
		fmt.Println("No agents found.")
		return nil
	}

	headers := []string{"ID", "Name", "Email", "Availability", "Role"}
	rows := make([][]string, 0, len(agents))
	for _, agent := range agents {
		rows = append(rows, []string{
			strconv.Itoa(agent.ID),
			agent.Name,
			agent.Email,
			agent.AvailabilityStatus,
			agent.Role,
		})
	}

	app.Printer.PrintTable(headers, rows)
	return nil
}
