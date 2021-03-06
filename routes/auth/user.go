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

package auth

import (
	"errors"
	"fmt"
	"net/url"

	user "github.com/MottainaiCI/mottainai-server/pkg/user"

	"github.com/MottainaiCI/mottainai-server/pkg/context"
	database "github.com/MottainaiCI/mottainai-server/pkg/db"
	userapi "github.com/MottainaiCI/mottainai-server/routes/api/user"

	setting "github.com/MottainaiCI/mottainai-server/pkg/settings"
	"github.com/go-macaron/captcha"
	log "gopkg.in/clog.v1"
)

const (
	LOGIN  = "user/auth/login"
	SIGNUP = "user/auth/signup"
	LIST   = "user/auth/list"
	SHOW   = "user/show"
)

// AutoLogin reads cookie and try to auto-login.
func AutoLogin(c *context.Context, db *database.Database) (bool, error) {

	uname := c.GetCookie("u_name")
	if len(uname) == 0 {
		return false, nil
	}

	isSucceed := false
	defer func() {
		if !isSucceed {
			log.Trace("auto-login cookie cleared: %s", uname)
			c.SetCookie("u_name", "", -1, db.Config.GetWeb().AppSubURL)
			c.SetCookie("r_name", "", -1, db.Config.GetWeb().AppSubURL)
			c.SetCookie("s_name", "", -1, db.Config.GetWeb().AppSubURL)
		}
	}()

	u, err := db.Driver.GetUserByName(uname)
	if err != nil {

		return false, fmt.Errorf("GetUserByName: %v", err)

	}

	if val, ok := c.GetSuperSecureCookie(u.Password, "r_name"); !ok || val != u.Name {
		return false, nil
	}

	isSucceed = true
	c.Session.Set("uid", u.ID)
	c.Session.Set("uname", u.Name)
	c.SetCookie("_csrf", "", -1, db.Config.GetWeb().AppSubURL)
	//	if db.Config.EnableLoginStatusCookie {
	//		c.SetCookie(setting.LoginStatusCookieName, "true", 0, db.Config.AppSubURL)
	//	}
	return true, nil
}

// isValidRedirect returns false if the URL does not redirect to same site.
// False: //url, http://url
// True: /url
func isValidRedirect(url string) bool {
	return len(url) >= 2 && url[0] == '/' && url[1] != '/'
}

func Login(c *context.Context, db *database.Database) {
	c.Title("Log in")

	// Check auto-login
	isSucceed, err := AutoLogin(c, db)
	if err != nil {
		c.ServerError("AutoLogin", err)
		return
	}

	redirectTo := c.Query("redirect_to")
	if len(redirectTo) > 0 {
		c.SetCookie("redirect_to", redirectTo, 0, db.Config.GetWeb().AppSubURL)
	} else {
		redirectTo, _ = url.QueryUnescape(c.GetCookie("redirect_to"))
	}

	if isSucceed {
		if isValidRedirect(redirectTo) {
			c.Redirect(redirectTo)
		} else {
			c.SubURLRedirect("/")
		}
		c.SetCookie("redirect_to", "", -1, db.Config.GetWeb().AppSubURL)
		return
	}
	uuu, err := db.Driver.GetSettingByKey(setting.SYSTEM_SIGNUP_ENABLED)
	if err == nil {
		if uuu.IsDisabled() {
			c.Data["NoSignUp"] = "yes"
		}
	}

	c.Success(LOGIN)
}

func afterLogin(c *context.Context, u user.User, remember bool) {
	var redirectTo string

	c.Invoke(func(config *setting.Config) {
		if remember {
			days := 86400 * 30
			c.SetCookie("u_name", u.Name, days, config.GetWeb().AppSubURL, "", true, true)
			c.SetSuperSecureCookie(u.Password, "r_name", u.Name, days, config.GetWeb().AppSubURL, "", true, true)
		}

		c.Session.Set("uid", u.ID)
		c.Session.Set("uname", u.Name)

		// Clear whatever CSRF has right now, force to generate a new one
		c.SetCookie("_csrf", "", -1, config.GetWeb().AppSubURL)

		redirectTo, _ = url.QueryUnescape(c.GetCookie("redirect_to"))
		c.SetCookie("redirect_to", "", -1, config.GetWeb().AppSubURL)
	})

	if isValidRedirect(redirectTo) {
		c.Redirect(redirectTo)
		return
	}

	c.SubURLRedirect("/")
}

func LoginPost(c *context.Context, f SignIn, db *database.Database) {
	var err error
	var u user.User
	c.Title("Sign in")

	if c.HasError() {
		c.Success(LOGIN)
		return
	}

	onlyuser_val, err := db.Driver.GetSettingByKey(
		setting.SYSTEM_SIGNIN_ONLY_USERVALIDATION)
	if err == nil {
		if onlyuser_val.IsEnabled() {
			u, err = db.Driver.GetUserByName(f.UserName)
		} else {
			u, err = db.Driver.SignIn(f.UserName, f.Password)
		}
	} else {
		// If setting is not present erro is No settingname found
		// I consider so that user and password validation is enable
		u, err = db.Driver.SignIn(f.UserName, f.Password)
	}

	if err != nil {
		c.RenderWithErr(err.Error(), LOGIN)
		//c.ServerError("UserLogin", err)
		return
	}

	afterLogin(c, u, f.Remember)
	return

}

func SignOut(c *context.Context) {
	c.Invoke(func(config *setting.Config) {
		c.Session.Delete("uid")
		c.Session.Delete("uname")
		c.SetCookie("u_name", "", -1, config.GetWeb().AppSubURL)
		c.SetCookie("r_name", "", -1, config.GetWeb().AppSubURL)
		c.SetCookie("_csrf", "", -1, config.GetWeb().AppSubURL)
		c.SubURLRedirect("/")
	})
}

func SignUp(c *context.Context, db *database.Database) {

	uuu, err := db.Driver.GetSettingByKey(setting.SYSTEM_SIGNUP_ENABLED)
	if err == nil {
		if uuu.IsDisabled() {
			c.ServerError("Signup disabled", errors.New("Signup disabled"))
			return
		}
	}

	c.Title("Sign up")

	c.Data["EnableCaptcha"] = true

	c.Success(SIGNUP)
}

func SetManager(ctx *context.Context, db *database.Database) {
	err := userapi.SetManager(ctx, db)
	if err != nil {
		ctx.ServerError("Failed to make user manager", err)
		return
	}

	ctx.SubURLRedirect("/user/list")
}

func SetAdmin(ctx *context.Context, db *database.Database) {
	err := userapi.SetAdmin(ctx, db)
	if err != nil {
		ctx.ServerError("Failed to make user admin", err)
		return
	}

	ctx.SubURLRedirect("/user/list")
}

func UnSetAdmin(ctx *context.Context, db *database.Database) {
	err := userapi.UnSetAdmin(ctx, db)
	if err != nil {
		ctx.ServerError("Failed removing user from admins", err)
		return
	}

	ctx.SubURLRedirect("/user/list")
}

func UnSetManager(ctx *context.Context, db *database.Database) {
	err := userapi.UnSetManager(ctx, db)
	if err != nil {
		ctx.ServerError("Failed removing user from managers", err)
		return
	}

	ctx.SubURLRedirect("/user/list")
}
func DeleteUser(ctx *context.Context, db *database.Database) {
	err := userapi.Delete(ctx, db)
	if err != nil {
		ctx.ServerError("Failed deleting user", err)
		return
	}

	ctx.SubURLRedirect("/user/list")
}

func SignUpPost(c *context.Context, cpt *captcha.Captcha, f Register, db *database.Database) {
	uuu, err := db.Driver.GetSettingByKey(setting.SYSTEM_SIGNUP_ENABLED)
	if err == nil {
		if uuu.IsDisabled() {
			c.ServerError("Signup disabled", errors.New("Signup disabled"))
			return
		}
	}
	c.Title("Sign Up")

	c.Data["EnableCaptcha"] = true

	if c.HasError() {
		c.Success(SIGNUP)
		return
	}
	//Captcha
	if !cpt.VerifyReq(c.Req) {
		c.RenderWithErr("Captcha verification failed", SIGNUP)
		return
	}

	if f.Password != f.Retype {
		c.RenderWithErr("Failed to type password twice", SIGNUP)
		return
	}

	//check := namespace.Namespace{Name: f.UserName}

	//if check.Exists() {
	//		c.RenderWithErr("Username taken as namespace, pick another one", SIGNUP)
	//		return
	//	}

	u := &user.User{
		Name:     f.UserName,
		Email:    f.Email,
		Password: f.Password,
		//IsActive: !setting.Service.RegisterEmailConfirm,
	}
	if db.Driver.CountUsers() == 0 {
		u.MakeAdmin() // XXX: ugly, also fix error
	}
	if _, err := db.Driver.InsertAndSaltUser(u); err != nil {
		c.RenderWithErr("Failed creating new user "+err.Error(), SIGNUP)
		return
	}
	log.Trace("Account created: %s", u.Name)

	c.SubURLRedirect("/user/login")
}

func ListUsers(c *context.Context, db *database.Database) {
	c.Data["Users"] = userapi.List(c, db)
	c.Success(LIST)
}

func Show(c *context.Context, db *database.Database) {

	u, err := userapi.Show(c, db)
	if err != nil {
		c.NotFound()
		return
	}
	c.Data["User"] = u
	c.Success(SHOW)
}
