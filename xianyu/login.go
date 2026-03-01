package xianyu

import (
	"context"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/pkg/errors"
)

const homeURL = "https://www.goofish.com/"

type LoginAction struct {
	page *rod.Page
}

func NewLogin(page *rod.Page) *LoginAction {
	return &LoginAction{page: page}
}

func (a *LoginAction) CheckLoginStatus(ctx context.Context) (bool, string, error) {
	pp := a.page.Context(ctx)
	pp.MustNavigate(homeURL).MustWaitLoad().MustWaitStable()
	return a.detectLoggedIn(pp)
}

func (a *LoginAction) Login(ctx context.Context) error {
	pp := a.page.Context(ctx)
	pp.MustNavigate(homeURL).MustWaitLoad().MustWaitStable()

	loggedIn, _, err := a.detectLoggedIn(pp)
	if err != nil {
		return err
	}
	if loggedIn {
		return nil
	}

	_ = a.tryOpenLoginDialog(pp)
	if !a.WaitForLogin(ctx) {
		return errors.New("login timeout or cancelled")
	}
	return nil
}

func (a *LoginAction) FetchQrcodeImage(ctx context.Context) (string, bool, error) {
	pp := a.page.Context(ctx)
	pp.MustNavigate(homeURL).MustWaitLoad().MustWaitStable()

	loggedIn, _, err := a.detectLoggedIn(pp)
	if err != nil {
		return "", false, err
	}
	if loggedIn {
		return "", true, nil
	}

	_ = a.tryOpenLoginDialog(pp)

	for i := 0; i < 8; i++ {
		src, err := a.extractQRCode(pp)
		if err == nil && src != "" {
			return src, false, nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return "", false, errors.New("qrcode src is empty")
}

func (a *LoginAction) WaitForLogin(ctx context.Context) bool {
	pp := a.page.Context(ctx)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			loggedIn, _, err := a.detectLoggedIn(pp)
			if err == nil && loggedIn {
				return true
			}
		}
	}
}

func (a *LoginAction) detectLoggedIn(pp *rod.Page) (bool, string, error) {
	result, err := pp.Eval(`() => {
		const visible = (el) => {
			if (!el) return false;
			const style = window.getComputedStyle(el);
			if (!style) return false;
			return style.display !== 'none' && style.visibility !== 'hidden' && el.offsetWidth > 0 && el.offsetHeight > 0;
		};

		const loginHints = ['扫码登录', '请登录', '立即登录', '账号登录', '手机号登录', '去登录', '登录'];
		const clickableNodes = Array.from(document.querySelectorAll('a,button,div,span')).filter(visible);
		const hasLoginButton = clickableNodes.some((el) => {
			const t = (el.textContent || '').trim();
			if (!t || t.length > 12) return false;
			return loginHints.some((k) => t === k || t.startsWith(k));
		});

		const avatarSelectors = [
			'[class*="avatar"] img',
			'img[class*="avatar"]',
			'[class*="user-avatar"] img',
			'[data-role*="avatar"]',
			'a[href*="profile"] img',
			'a[href*="my"] img'
		];
		let hasAvatar = false;
		for (const sel of avatarSelectors) {
			const el = document.querySelector(sel);
			if (visible(el)) {
				hasAvatar = true;
				break;
			}
		}

		const nicknameSelectors = [
			'[class*="nickname"]',
			'[class*="user-name"]',
			'[class*="username"]',
			'[data-role*="nickname"]'
		];
		let nickname = '';
		for (const sel of nicknameSelectors) {
			const el = document.querySelector(sel);
			if (visible(el)) {
				const t = (el.textContent || '').trim();
				if (t.length >= 2 && t.length <= 30) {
					nickname = t;
					break;
				}
			}
		}

		const loggedIn = hasAvatar && !hasLoginButton;
		return { loggedIn, nickname };
	}`)
	if err != nil {
		return false, "", errors.Wrap(err, "check login status failed")
	}

	loggedIn := result.Value.Get("loggedIn").Bool()
	nickname := strings.TrimSpace(result.Value.Get("nickname").String())
	return loggedIn, nickname, nil
}

func (a *LoginAction) tryOpenLoginDialog(pp *rod.Page) error {
	_, err := pp.Eval(`() => {
		const visible = (el) => {
			if (!el) return false;
			const style = window.getComputedStyle(el);
			return style.display !== 'none' && style.visibility !== 'hidden';
		};

		const candidates = Array.from(document.querySelectorAll('a,button,div,span')).filter(visible);
		const el = candidates.find((item) => {
			const t = (item.textContent || '').trim();
			return ['登录', '立即登录', '请登录', '扫码登录'].some((k) => t === k || t.includes(k));
		});
		if (el && typeof el.click === 'function') {
			el.click();
			return true;
		}
		return false;
	}`)
	if err != nil {
		return errors.Wrap(err, "open login dialog failed")
	}
	return nil
}

func (a *LoginAction) extractQRCode(pp *rod.Page) (string, error) {
	result, err := pp.Eval(`() => {
		const visible = (el) => {
			if (!el) return false;
			const style = window.getComputedStyle(el);
			if (!style) return false;
			return style.display !== 'none' && style.visibility !== 'hidden' && el.offsetWidth > 60 && el.offsetHeight > 60;
		};

		const selectors = [
			'img.qrcode-img',
			'img[class*="qrcode"]',
			'img[class*="qr-code"]',
			'img[src*="qrcode"]',
			'img[src*="qr"]',
			'.qrcode img',
			'.qr-code img'
		];

		for (const sel of selectors) {
			const el = document.querySelector(sel);
			if (visible(el)) {
				const src = el.getAttribute('src') || el.src || '';
				if (src) return { src };
			}
		}

		const canvases = Array.from(document.querySelectorAll('canvas')).filter(visible);
		for (const canvas of canvases) {
			if (canvas.width >= 120 && canvas.height >= 120) {
				try {
					return { src: canvas.toDataURL('image/png') };
				} catch (e) {
				}
			}
		}

		return { src: '' };
	}`)
	if err != nil {
		return "", errors.Wrap(err, "get qrcode src failed")
	}

	src := strings.TrimSpace(result.Value.Get("src").String())
	if src == "" {
		return "", errors.New("qrcode src is empty")
	}
	if strings.HasPrefix(src, "//") {
		src = "https:" + src
	}
	if strings.HasPrefix(src, "/") {
		src = "https://www.goofish.com" + src
	}
	return src, nil
}
