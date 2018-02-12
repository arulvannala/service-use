package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/cdelashmutt-pivotal/service-use/apihelper"
	"github.com/cloudfoundry/cli/cf/terminal"
	"github.com/cloudfoundry/cli/plugin"
)

type ServiceUsePlugin struct {
	apiHelper apihelper.CFAPIHelper
	cli       plugin.CliConnection
	ui        terminal.UI
}

// contains CLI flag values
type flagVal struct {
	Format string
}

func ParseFlags(args []string) flagVal {
	flagSet := flag.NewFlagSet(args[0], flag.ContinueOnError)

	// Create flags
	format := flagSet.String("f", "format", "-f <csv>")

	err := flagSet.Parse(args[1:])
	if err != nil {

	}

	return flagVal{
		Format: string(*format),
	}
}

type report struct {
	services []service
}

func (cmd *ServiceUsePlugin) ServiceUseCommand(args []string) {
	var aReport report

	flagVals := ParseFlags(args)

	if nil == cmd.cli {
		fmt.Println("ERROR: CLI Connection is nil!")
		os.Exit(1)
	}

	loggedIn, err := cmd.cli.IsLoggedIn()

	if nil != err {
		fmt.Printf("Error checking if you are logged in: %s\n", err)
		os.Exit(1)
	}

	if !loggedIn {
		fmt.Printf("Please login before trying to run this command.\n")
		os.Exit(1)
	}

	services, _ := cmd.getServices()

	aReport.services = services

	username, _ := cmd.cli.Username()

	if flagVals.Format == "csv" {
		fmt.Println(aReport.CSV())
	} else {
		fmt.Println(aReport.String(username))
	}
}

func (aReport *report) String(username string) string {
	var response bytes.Buffer

	response.WriteString(fmt.Sprintf("Getting service use information as %s...\n\n",
		terminal.EntityNameColor(username)))

	for _, service := range aReport.services {
		response.WriteString(fmt.Sprintf("Service %s:\n", terminal.EntityNameColor(service.label)))
		for _, serviceplan := range service.plans {
			response.WriteString(fmt.Sprintf(" Plan %s:\n", terminal.EntityNameColor(serviceplan.name)))

			for _, serviceinstance := range serviceplan.serviceinstances {
				response.WriteString(
					fmt.Sprintf("  Org: %s, Space: %s, Instance: %s, Managers: [%s]\n",
						terminal.EntityNameColor(serviceinstance.space.organization.name),
						terminal.EntityNameColor(serviceinstance.space.name),
						terminal.EntityNameColor(serviceinstance.name),
						strings.Join(serviceinstance.space.organization.managers, ",")))
			}
		}
	}

	return response.String()
}

func (aReport *report) CSV() string {
	var rows = [][]string{}
	var csv bytes.Buffer

	var headers = []string{"ServiceName", "PlanName", "OrgName", "SpaceName", "InstanceName", "Managers"}

	rows = append(rows, headers)

	for _, service := range aReport.services {
		for _, plan := range service.plans {
			for _, serviceinstance := range plan.serviceinstances {
				serviceResult := []string{
					service.label,
					plan.name,
					serviceinstance.space.organization.name,
					serviceinstance.space.name,
					serviceinstance.name,
					strings.Join(serviceinstance.space.organization.managers, "|"),
				}

				rows = append(rows, serviceResult)
			}
		}
	}

	for i := range rows {
		csv.WriteString(strings.Join(rows[i], ", "))
		csv.WriteString("\n")
	}

	return csv.String()
}

type service struct {
	label string
	plans []serviceplan
}

func (cmd *ServiceUsePlugin) getServices() ([]service, error) {
	rawServices, err := cmd.apiHelper.GetServices()
	if nil != err {
		return nil, err
	}

	var services = []service{}

	for _, s := range rawServices {

		serviceplans, err := cmd.getServicePlans(s.ServicePlansURL)
		if nil != err {
			return nil, err
		}

		services = append(services, service{
			label: s.Label,
			plans: serviceplans,
		})
	}
	return services, nil
}

type serviceplan struct {
	name             string
	serviceinstances []serviceinstance
}

func (cmd *ServiceUsePlugin) getServicePlans(servicePlansURL string) ([]serviceplan, error) {
	rawServicePlans, err := cmd.apiHelper.GetServicePlans(servicePlansURL)
	if nil != err {
		return nil, err
	}

	var serviceplans = []serviceplan{}

	for _, sp := range rawServicePlans {

		serviceinstances, err := cmd.getServiceInstances(sp.ServiceInstancesURL)
		if nil != err {
			return nil, err
		}
		serviceplans = append(serviceplans, serviceplan{
			name:             sp.Name,
			serviceinstances: serviceinstances,
		})
	}
	return serviceplans, nil
}

type serviceinstance struct {
	name  string
	space space
}

func (cmd *ServiceUsePlugin) getServiceInstances(serviceInstancesURL string) ([]serviceinstance, error) {
	rawServiceInstances, err := cmd.apiHelper.GetServiceInstances(serviceInstancesURL)
	if nil != err {
		return nil, err
	}

	var serviceinstances = []serviceinstance{}

	for _, si := range rawServiceInstances {

		space, err := cmd.getSpace(si.SpaceURL)
		if nil != err {
			return nil, err
		}

		serviceinstances = append(serviceinstances, serviceinstance{
			name:  si.Name,
			space: space,
		})
	}
	return serviceinstances, nil
}

type space struct {
	name         string
	organization organization
}

func (cmd *ServiceUsePlugin) getSpace(spaceURL string) (space, error) {

	rawSpace, err := cmd.apiHelper.GetSpace(spaceURL)
	if nil != err {
		return space{}, err
	}

	organization, err := cmd.getOrganization(rawSpace.OrganizationURL)
	if nil != err {
		return space{}, err
	}

	space := space{
		name:         rawSpace.Name,
		organization: organization,
	}
	return space, nil
}

type organization struct {
	name     string
	managers []string
}

var orgCache map[string]organization = make(map[string]organization)

func (cmd *ServiceUsePlugin) getOrganization(organizationURL string) (organization, error) {
	if retOrg, present := orgCache[organizationURL]; !present {
		retOrg, _ = cmd.actualGetOrganization(organizationURL)
		orgCache[organizationURL] = retOrg
	}
	return orgCache[organizationURL], nil
}

func (cmd *ServiceUsePlugin) actualGetOrganization(organizationURL string) (organization, error) {
	rawOrg, err := cmd.apiHelper.GetOrganization(organizationURL)
	if nil != err {
		return organization{}, err
	}

	orgManagers, err := cmd.getOrgManagers(rawOrg.ManagersURL)

	organization := organization{
		name:     rawOrg.Name,
		managers: orgManagers,
	}
	return organization, nil
}

func (cmd *ServiceUsePlugin) getOrgManagers(orgManagersURL string) ([]string, error) {
	rawOrgManagers, err := cmd.apiHelper.GetOrgManagers(orgManagersURL)
	if nil != err {
		return nil, err
	}

	var orgmanagers = []string{}

	for _, om := range rawOrgManagers {
		orgmanagers = append(orgmanagers, om.UserName)
	}
	return orgmanagers, nil
}

func (cmd *ServiceUsePlugin) Run(cli plugin.CliConnection, args []string) {

	if args[0] == "service-use" {
		cmd.cli = cli
		cmd.apiHelper = apihelper.New(cli)
		cmd.ServiceUseCommand(args)
	}
}

func (cmd *ServiceUsePlugin) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name: "ServiceUsePlugin",
		Version: plugin.VersionType{
			Major: 1,
			Minor: 2,
			Build: 1,
		},
		Commands: []plugin.Command{
			plugin.Command{
				Name:     "service-use",
				HelpText: "Infomation about service instances and bound apps",

				// UsageDetails is optional
				// It is used to show help of usage of each command
				UsageDetails: plugin.Usage{
					Usage: "cf service-use [-f csv]",
					Options: map[string]string{
						"f": "format output as csv",
					},
				},
			},
		},
	}
}

func main() {
	plugin.Start(new(ServiceUsePlugin))
}
