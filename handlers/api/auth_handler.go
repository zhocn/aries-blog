package api

import (
	"aries/config/setting"
	"aries/forms"
	"aries/models"
	"aries/utils"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/go-gomail/gomail"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

type AuthHandler struct {
}

// @Summary 注册
// @Tags 授权
// @version 1.0
// @Accept application/json
// @Param regForm body form.RegisterForm true "注册表单"
// @Success 100 object util.Result 成功
// @Failure 103/104 object util.Result 失败
// @Router /api/v1/auth/register [post]
func (a *AuthHandler) Register(ctx *gin.Context) {
	regForm := forms.RegisterForm{}
	result := utils.Result{ // 定义 api 返回信息结构
		Code: utils.Success,
		Msg:  "注册成功",
		Data: nil,
	}
	if err := ctx.ShouldBindJSON(&regForm); err != nil { // 表单校验失败
		result.Code = utils.RequestError     // 请求数据有误
		result.Msg = utils.GetFormError(err) // 获取表单错误信息
		ctx.JSON(http.StatusOK, result)      // 返回 json
		return
	}
	user := regForm.BindToModel() // 绑定表单数据到用户
	u, _ := user.GetByUsername()  // 根据用户名获取用户
	if u.Username != "" {         // 账号已被注册
		result.Code = utils.RequestError
		result.Msg = "该用户已被注册"
		ctx.JSON(http.StatusOK, result) // 返回 json
		return
	}
	if err := user.Create(); err != nil { // 创建用户 + 异常处理
		log.Errorln("error: ", err.Error())
		result.Code = utils.ServerError
		result.Msg = "服务器端错误"
		ctx.JSON(http.StatusOK, result) // 返回 json
		return
	}
	sysSetting := models.SysSetting{Name: "网站设置"}
	if err := sysSetting.Create(); err != nil {
		log.Errorln("error: ", err.Error())
		result.Code = utils.ServerError
		result.Msg = "服务器端错误"
		ctx.JSON(http.StatusOK, result)
		return
	}
	typeItem := models.SysSettingItem{
		SysId: sysSetting.ID,
		Key:   "type_name",
		Val:   "网站设置",
	}
	siteUrlItem := models.SysSettingItem{
		SysId: sysSetting.ID,
		Key:   "site_url",
		Val:   regForm.SiteUrl,
	}
	siteNameItem := models.SysSettingItem{
		SysId: sysSetting.ID,
		Key:   "site_name",
		Val:   regForm.SiteName,
	}
	itemList := []models.SysSettingItem{typeItem, siteNameItem, siteUrlItem}
	err := models.SysSettingItem{}.MultiCreateOrUpdate(sysSetting.ID, itemList)
	if err != nil {
		log.Errorln("error: ", err.Error())
		result.Code = utils.ServerError
		result.Msg = "服务器端错误"
		ctx.JSON(http.StatusOK, result)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

// @Summary 登录
// @Tags 授权
// @version 1.0
// @Accept application/json
// @Param loginForm body form.LoginForm true "登录表单"
// @Success 100 object util.Result 成功
// @Failure 103/104 object util.Result 失败
// @Router /api/v1/auth/login [post]
func (a *AuthHandler) Login(ctx *gin.Context) {
	loginForm := forms.LoginForm{}
	result := utils.Result{ // 定义 api 返回信息结构
		Code: utils.Success,
		Msg:  "登录成功",
		Data: nil,
	}
	if err := ctx.ShouldBindJSON(&loginForm); err != nil { // 表单校验失败
		result.Code = utils.RequestError     // 请求数据有误
		result.Msg = utils.GetFormError(err) // 获取表单错误信息
		ctx.JSON(http.StatusOK, result)      // 返回 json
		return
	}
	captchaConfig := &utils.CaptchaConfig{
		Id:          loginForm.CaptchaId,
		VerifyValue: loginForm.CaptchaVal,
	}
	if !utils.CaptchaVerify(captchaConfig) { // 校验验证码
		result.Code = utils.RequestError // 请求数据有误
		result.Msg = "验证码错误"
		ctx.JSON(http.StatusOK, result) // 返回 json
		return
	}
	user := loginForm.BindToModel() // 绑定表单数据到实体类
	u, _ := user.GetByUsername()    // 根据用户名获取用户
	if u.Username == "" {           // 用户不存在
		result.Code = utils.RequestError
		result.Msg = "不存在该用户"
		ctx.JSON(http.StatusOK, result)
		return
	}
	if !utils.VerifyPwd(u.Pwd, user.Pwd) { // 密码错误
		result.Code = utils.RequestError
		result.Msg = "密码错误"
		ctx.JSON(http.StatusOK, result) // 返回 json
		return
	}
	j := utils.NewJWT()                             // 创建 JWT 实例
	token, err := j.CreateToken(utils.CustomClaims{ // 生成 JWT token
		Username: u.Username,
		UserImg:  u.UserImg,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Second * time.
				Duration(setting.Config.Server.TokenExpireTime)).Unix(), // 设置过期时间
			IssuedAt: time.Now().Unix(),
		},
	})
	if err != nil { // 异常处理
		log.Errorln("error: ", err.Error())
		result.Code = utils.ServerError
		result.Msg = "服务器端错误"
		ctx.JSON(http.StatusOK, result) // 返回 json
		return
	}
	result.Data = utils.Token{ // 封装 Token 信息
		Token:    token,
		UserId:   u.ID,
		Username: u.Username,
		UserImg:  u.UserImg,
	}
	ctx.JSON(http.StatusOK, result) // 返回 json
}

// @Summary 创建验证码
// @Tags 授权
// @version 1.0
// @Accept application/json
// @Success 100 object util.Result 成功
// @Failure 103/104 object util.Result 失败
// @Router /api/v1/auth/captcha [get]
func (a *AuthHandler) CreateCaptcha(ctx *gin.Context) {
	captcha := utils.CaptchaConfig{} // 创建验证码配置结构
	result := utils.Result{          // 返回数据结构
		Code: utils.Success,
		Msg:  "验证码创建成功",
		Data: nil,
	}
	base64, err := utils.GenerateCaptcha(&captcha) // 创建验证码
	if err != nil {                                // 异常处理
		result.Code = utils.ServerError
		result.Msg = "服务器端错误"
		ctx.JSON(http.StatusOK, result)
		return
	}
	result.Data = gin.H{ // 封装 data
		"captcha_id":  captcha.Id,
		"captcha_url": base64,
	}
	ctx.JSON(http.StatusOK, result) // 返回 json 数据
}

// @Summary 忘记密码
// @Tags 授权
// @version 1.0
// @Accept application/json
// @Param forgetPwdForm body form.ForgetPwdForm true "忘记密码表单"
// @Success 100 object util.Result 成功
// @Failure 103/104 object util.Result 失败
// @Router /api/v1/auth/pwd/forget [post]
func (a *AuthHandler) ForgetPwd(ctx *gin.Context) {
	forgetPwdForm := forms.ForgetPwdForm{}
	if err := ctx.ShouldBindJSON(&forgetPwdForm); err != nil {
		ctx.JSON(http.StatusOK, utils.Result{
			Code: utils.RequestError,
			Msg:  utils.GetFormError(err),
			Data: nil,
		})
		return
	}
	user, _ := models.User{Email: forgetPwdForm.Email}.GetByEmail()
	if user.Username == "" {
		ctx.JSON(http.StatusOK, utils.Result{
			Code: utils.RequestError,
			Msg:  "不存在该邮箱帐号",
			Data: nil,
		})
		return
	}
	verifyCode := ""
	_ = setting.Cache.Get(forgetPwdForm.Email, &verifyCode)
	if verifyCode == "" {
		verifyCode = utils.CreateRandomCode(6)
		_ = setting.Cache.Set(forgetPwdForm.Email, verifyCode, time.Minute*15)
	}
	msg := gomail.NewMessage()
	// 设置收件人
	msg.SetHeader("To", forgetPwdForm.Email)
	// 设置发件人
	msg.SetAddressHeader("From", setting.Config.SMTP.Account, setting.Config.SMTP.Account)
	// 主题
	msg.SetHeader("Subject", "忘记密码验证")
	// 正文
	msg.SetBody("text/html", utils.GetForgetPwdEmailHTML(user.Username, verifyCode))
	// 设置 SMTP 参数
	d := gomail.NewDialer(setting.Config.SMTP.Address, setting.Config.SMTP.Port,
		setting.Config.SMTP.Account, setting.Config.SMTP.Password)
	// 发送
	err := d.DialAndSend(msg)
	if err != nil {
		log.Error("验证码发送失败：", err.Error())
		ctx.JSON(http.StatusOK, utils.Result{
			Code: utils.ServerError,
			Msg:  "验证码发送失败，请检查 smtp 配置",
			Data: nil,
		})
		return
	}
	ctx.JSON(http.StatusOK, utils.Result{
		Code: utils.Success,
		Msg:  "验证码发送成功，请前往邮箱查看",
		Data: nil,
	})
}

// @Summary 重置密码
// @Tags 授权
// @version 1.0
// @Accept application/json
// @Param resetPwdForm body form.ResetPwdForm true "重置密码表单"
// @Success 100 object util.Result 成功
// @Failure 103/104 object util.Result 失败
// @Router /api/v1/auth/pwd/reset [post]
func (a *AuthHandler) ResetPwd(ctx *gin.Context) {
	resetPwdForm := forms.ResetPwdForm{}
	if err := ctx.ShouldBindJSON(&resetPwdForm); err != nil {
		ctx.JSON(http.StatusOK, utils.Result{
			Code: utils.RequestError,
			Msg:  utils.GetFormError(err),
			Data: nil,
		})
		return
	}
	verifyCode := ""
	_ = setting.Cache.Get(resetPwdForm.Email, &verifyCode)
	if verifyCode != resetPwdForm.VerifyCode {
		ctx.JSON(http.StatusOK, utils.Result{
			Code: utils.RequestError,
			Msg:  "验证码无效或错误",
			Data: nil,
		})
		return
	}
	user := resetPwdForm.BindToModel()
	err := user.UpdatePwd()
	if err != nil {
		log.Error("error: ", err.Error())
		ctx.JSON(http.StatusOK, utils.Result{
			Code: utils.ServerError,
			Msg:  "服务器端错误",
			Data: nil,
		})
		return
	}
	ctx.JSON(http.StatusOK, utils.Result{
		Code: utils.Success,
		Msg:  "重置密码成功",
		Data: nil,
	})
}