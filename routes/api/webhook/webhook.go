/*

Copyright (C) 2017-2018  Ettore Di Giacinto <mudler@gentoo.org>
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

package apiwebhook

import (
	"errors"

	"github.com/MottainaiCI/mottainai-server/pkg/context"
	database "github.com/MottainaiCI/mottainai-server/pkg/db"
	setting "github.com/MottainaiCI/mottainai-server/pkg/settings"

	macaron "gopkg.in/macaron.v1"
)

func RequiresWebHookSetting(c *context.Context, db *database.Database) error {
	// Check setting if we have to process this.
	err := errors.New("Webhook integration disabled")
	uuu, err := db.Driver.GetSettingByKey(setting.SYSTEM_WEBHOOK_ENABLED)
	if err == nil {
		if uuu.IsDisabled() {
			c.ServerError("Webhook integration disabled", err)
			return err
		}
	}
	return nil
}

func Setup(m *macaron.Macaron) {
	reqSignIn := context.Toggle(&context.ToggleOptions{SignInRequired: true})

	m.Get("/api/webhook", RequiresWebHookSetting, reqSignIn, ShowAll)
	m.Get("/api/webhook/create/:type", RequiresWebHookSetting, reqSignIn, Create)
	m.Get("/api/webhook/delete/:id", RequiresWebHookSetting, reqSignIn, Remove)
}