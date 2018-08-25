/*

Copyright (C) 2018  Ettore Di Giacinto <mudler@gentoo.org>
Credits goes also to Gogs authors, some code portions and re-implemented design
are also coming from the Gogs project, which is using the go-macaron framework
and was really source of ispiration. Kudos to them!

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.

*/

package mottainai

import (
	"os"
	"path"

	setting "github.com/MottainaiCI/mottainai-server/pkg/settings"
	agenttasks "github.com/MottainaiCI/mottainai-server/pkg/tasks"
	"github.com/MottainaiCI/mottainai-server/pkg/utils"
	"github.com/RichardKnop/machinery/v1/log"

	client "github.com/MottainaiCI/mottainai-server/pkg/client"
)

func (m *MottainaiAgent) HealthCheckSetup() {
	th := agenttasks.DefaultTaskHandler()
	m.Map(th)
	//ID := utils.GenID()
	//hostname := utils.Hostname()
	//log.INFO.Println("Worker ID: " + ID)
	//log.INFO.Println("Worker Hostname: " + hostname)

	fetcher := client.NewClient(setting.Configuration.AppURL)
	fetcher.Token = setting.Configuration.ApiKey

	//fetcher.RegisterNode(ID, hostname)
	m.Map(fetcher)

	m.TimerSeconds(int64(800), true, func() { m.HealthClean() })
}

func (m *MottainaiAgent) HealthClean() {
	m.CleanBuildDir()

	m.Invoke(func(c *client.Fetcher) {

		var tlist []agenttasks.Task
		c.GetJSONOptions("/api/nodes/tasks/"+setting.Configuration.AgentKey, map[string]string{}, &tlist)
		for _, t := range tlist {
			if t.IsRunning() {
				log.INFO.Println("> Task running on the host, skipping deep host cleanup")
				return
			}
		}

		m.CleanHealthCheckPathHost()
		m.CleanDockerHost()
	})
}

// FIXME: temp (racy) workaround
// As vagrant does not guarantee removal of imported boxes, cleanup periodically
func (m *MottainaiAgent) CleanHealthCheckPathHost() {
	for _, k := range setting.Configuration.HealthCheckCleanPath {
		log.INFO.Println("> Removing dangling files in " + k)
		if err := utils.RemoveContents(k); err != nil {
			log.ERROR.Println("> Failed removing contents from ", k, " ", err.Error())
		}
	}
}

// FIXME: temp (racy) workaround
// Need to take care periodically of leaks that are generated by did tasks
func (m *MottainaiAgent) CleanDockerHost() {
	out, stderr, err := utils.Cmd("docker", []string{"system", "prune", "--force", "--all", "--volumes"})
	if err != nil {
		log.ERROR.Println("!! There was an error running the command: ", err.Error()+": "+stderr)
	}
	log.INFO.Println(out)
}

func (m *MottainaiAgent) CleanBuildDir() {
	m.Invoke(func(c *client.Fetcher) {
		log.INFO.Println("Cleaning " + setting.Configuration.BuildPath)

		stuff, err := utils.ListAll(setting.Configuration.BuildPath)
		if err != nil {
			panic(err)
		}

		defer func() {
			if r := recover(); r != nil {
				log.ERROR.Println(r)
			}
		}()

		for _, what := range stuff {
			c.Doc(what)
			th := agenttasks.DefaultTaskHandler()
			task_info := th.FetchTask(c)
			log.INFO.Println("Found: " + what)
			log.INFO.Println(task_info)
			if task_info.IsDone() || task_info.ID == "" {
				log.INFO.Println("Removing: " + what)
				os.RemoveAll(path.Join(setting.Configuration.BuildPath, what))
			} else {
				log.INFO.Println("Keeping: " + what)
			}
		}

	})
}
