package xianyu

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

const publishURL = "https://www.goofish.com/publish"

type PublishItemAction struct {
	page *rod.Page
}

type PublishItemContent struct {
	Images            []string `json:"images"`
	Description       string   `json:"description"`
	Price             string   `json:"price"`
	OriginalPrice     string   `json:"original_price,omitempty"`
	ShippingType      string   `json:"shipping_type,omitempty"` // 包邮|按距离计费|一口价|无需邮寄
	ShippingFee       string   `json:"shipping_fee,omitempty"`
	SupportSelfPickup bool     `json:"support_self_pickup,omitempty"`
	LocationKeyword   string   `json:"location_keyword,omitempty"`
	SpecTypes         []string `json:"spec_types,omitempty"`
	Submit            bool     `json:"submit,omitempty"`
}

type PublishItemResult struct {
	Submitted          bool     `json:"submitted"`
	PublishButtonReady bool     `json:"publish_button_ready"`
	CurrentURL         string   `json:"current_url"`
	DetectedShipping   string   `json:"detected_shipping,omitempty"`
	DetectedLocation   string   `json:"detected_location,omitempty"`
	UploadedImageCount int      `json:"uploaded_image_count"`
	Warnings           []string `json:"warnings,omitempty"`
}

func NewPublishItemAction(page *rod.Page) *PublishItemAction {
	return &PublishItemAction{
		page: page.Timeout(90 * time.Second),
	}
}

func (a *PublishItemAction) Publish(ctx context.Context, req PublishItemContent) (*PublishItemResult, error) {
	pp := a.page.Context(ctx)
	pp.MustNavigate(publishURL).MustWaitLoad()
	time.Sleep(2500 * time.Millisecond)

	if len(req.Images) == 0 {
		return nil, fmt.Errorf("images is required")
	}
	if strings.TrimSpace(req.Description) == "" {
		return nil, fmt.Errorf("description is required")
	}
	if strings.TrimSpace(req.Price) == "" {
		return nil, fmt.Errorf("price is required")
	}

	result := &PublishItemResult{
		Warnings: make([]string, 0),
	}

	validPaths := make([]string, 0, len(req.Images))
	for _, path := range req.Images {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			result.Warnings = append(result.Warnings, "图片不存在: "+path)
			continue
		}
		validPaths = append(validPaths, path)
	}
	if len(validPaths) == 0 {
		return nil, fmt.Errorf("no valid image file found")
	}

	fileInput, err := pp.Element(`input[type="file"][name="file"]`)
	if err != nil || fileInput == nil {
		return nil, fmt.Errorf("publish image input not found: %w", err)
	}
	if err := fileInput.SetFiles(validPaths); err != nil {
		return nil, fmt.Errorf("set publish images failed: %w", err)
	}
	time.Sleep(3 * time.Second)

	descriptionOK := pp.MustEval(`(text) => {
		const editor = document.querySelector('div[contenteditable="true"][class*="editor--"]') ||
			document.querySelector('div[contenteditable="true"]');
		if (!editor) return false;
		editor.focus();
		editor.innerText = text;
		editor.dispatchEvent(new Event('input', { bubbles: true }));
		editor.dispatchEvent(new Event('change', { bubbles: true }));
		return true;
	}`, req.Description).Bool()
	if !descriptionOK {
		return nil, fmt.Errorf("publish description editor not found")
	}
	time.Sleep(300 * time.Millisecond)

	if !fillPublishInputByLabel(pp, "价格", req.Price) {
		return nil, fmt.Errorf("publish price input not found")
	}
	if req.OriginalPrice != "" {
		if !fillPublishInputByLabel(pp, "原价", req.OriginalPrice) {
			result.Warnings = append(result.Warnings, "原价输入框未找到，已跳过")
		}
	}

	shipping := strings.TrimSpace(req.ShippingType)
	if shipping == "" {
		shipping = "包邮"
	}
	shippingValue, ok := mapShippingTypeToValue(shipping)
	if !ok {
		return nil, fmt.Errorf("unsupported shipping_type: %s", req.ShippingType)
	}

	switched := pp.MustEval(`(val) => {
		const radios = Array.from(document.querySelectorAll('input.ant-radio-input[type="radio"]'));
		for (const radio of radios) {
			if ((radio.value || '') === val) {
				radio.click();
				radio.dispatchEvent(new Event('change', { bubbles: true }));
				return true;
			}
		}
		return false;
	}`, shippingValue).Bool()
	if !switched {
		result.Warnings = append(result.Warnings, "发货方式切换失败，使用页面默认值")
	}
	time.Sleep(300 * time.Millisecond)

	if req.SupportSelfPickup {
		_ = pp.MustEval(`(enable) => {
			const nodes = Array.from(document.querySelectorAll('div,span'));
			const label = nodes.find((el) => (el.textContent || '').trim() === '支持自提');
			const switchBtn = (label && label.parentElement && label.parentElement.querySelector('button[role="switch"]'))
				|| document.querySelector('button[role="switch"]');
			if (!switchBtn) return false;
			const checked = switchBtn.getAttribute('aria-checked') === 'true';
			if (checked !== enable) switchBtn.click();
			return true;
		}`, true)
	}

	if req.ShippingFee != "" {
		if !fillPublishInputByLabel(pp, "邮费", req.ShippingFee) {
			result.Warnings = append(result.Warnings, "邮费输入框未找到，已跳过")
		}
	}

	addressOpened := pp.MustEval(`() => {
		const wrap = document.querySelector('div[class*="addressWrap"]') || document.querySelector('div[class*="address--"]');
		if (!wrap) return false;
		wrap.click();
		return true;
	}`).Bool()
	if !addressOpened {
		result.Warnings = append(result.Warnings, "宝贝所在地入口未找到")
	} else {
		time.Sleep(1200 * time.Millisecond)
		locationOK := pp.MustEval(`(keyword) => {
			const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
			const items = Array.from(document.querySelectorAll('div[class*="addressItem--"],div[class*="addressDesc--"]'));
			if (!items.length) return false;
			let target = null;
			const kw = clean(keyword);
			if (kw) {
				target = items.find((el) => clean(el.textContent).includes(kw)) || null;
			}
			if (!target) target = items[0];
			target.click();
			return true;
		}`, req.LocationKeyword).Bool()
		if !locationOK {
			result.Warnings = append(result.Warnings, "地址选择失败，可能需要手动选择")
		}
		time.Sleep(1200 * time.Millisecond)
	}

	if len(req.SpecTypes) > 0 {
		for _, spec := range req.SpecTypes {
			spec = strings.TrimSpace(spec)
			if spec == "" {
				continue
			}
			ok := pp.MustEval(`(name) => {
				const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
				const addBtn = document.querySelector('button[class*="addBtn"]') || document.querySelector('div[class*="addBtnContainer"]');
				if (!addBtn) return false;
				addBtn.click();
				const options = Array.from(document.querySelectorAll('div,span')).filter((el) => {
					const t = clean(el.textContent);
					return t && t.length <= 30;
				});
				const target = options.find((el) => clean(el.textContent) === name || clean(el.textContent).includes(name));
				if (!target) return false;
				target.click();
				return true;
			}`, spec).Bool()
			if !ok {
				result.Warnings = append(result.Warnings, "规格类型未匹配: "+spec)
			}
			time.Sleep(300 * time.Millisecond)
		}
	}

	snapshot := pp.MustEval(`() => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const publishBtn = Array.from(document.querySelectorAll('button')).find((el) => clean(el.textContent) === '发布' || clean(el.textContent).includes('发布'));
		const ready = !!publishBtn && !publishBtn.disabled && !(publishBtn.className || '').includes('disabled');
		const selectedShipping = clean(document.querySelector('label.ant-radio-wrapper-checked span:last-child')?.textContent || '');
		const location = clean(document.querySelector('div[class*="addressWrap"],div[class*="address--"]')?.textContent || '');
		const uploaded = document.querySelectorAll('div[class*="upload-item"],div[class*="picture--"]').length;
		return JSON.stringify({
			ready,
			selectedShipping,
			location,
			uploaded,
		});
	}`).String()

	type snapshotInfo struct {
		Ready            bool   `json:"ready"`
		SelectedShipping string `json:"selectedShipping"`
		Location         string `json:"location"`
		Uploaded         int    `json:"uploaded"`
	}
	var info snapshotInfo
	_ = json.Unmarshal([]byte(snapshot), &info)

	result.PublishButtonReady = info.Ready
	result.DetectedShipping = info.SelectedShipping
	result.DetectedLocation = info.Location
	result.UploadedImageCount = len(validPaths)
	result.CurrentURL = pp.MustInfo().URL

	if req.Submit {
		if !result.PublishButtonReady {
			return nil, fmt.Errorf("publish button is disabled after filling form")
		}

		clicked := pp.MustEval(`() => {
			const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
			const btn = Array.from(document.querySelectorAll('button')).find((el) => clean(el.textContent) === '发布' || clean(el.textContent).includes('发布'));
			if (!btn) return false;
			btn.click();
			return true;
		}`).Bool()
		if !clicked {
			return nil, fmt.Errorf("failed to click publish button")
		}

		time.Sleep(4 * time.Second)
		result.Submitted = true
		result.CurrentURL = pp.MustInfo().URL
	}

	return result, nil
}

func mapShippingTypeToValue(shippingType string) (string, bool) {
	switch strings.TrimSpace(shippingType) {
	case "包邮":
		return "0", true
	case "按距离计费":
		return "1", true
	case "一口价":
		return "2", true
	case "无需邮寄":
		return "3", true
	default:
		return "", false
	}
}

func fillPublishInputByLabel(pp *rod.Page, labelText, value string) bool {
	return pp.MustEval(`(labelText, value) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const items = Array.from(document.querySelectorAll('.ant-form-item'));
		for (const item of items) {
			const label = clean(item.querySelector('.ant-form-item-label')?.textContent || '');
			if (!label || !label.includes(labelText)) continue;
			const input = item.querySelector('input.ant-input,input[type="text"]');
			if (!input) continue;
			input.focus();
			input.value = '';
			input.dispatchEvent(new Event('input', { bubbles: true }));
			input.value = value;
			input.dispatchEvent(new Event('input', { bubbles: true }));
			input.dispatchEvent(new Event('change', { bubbles: true }));
			return true;
		}
		return false;
	}`, labelText, value).Bool()
}
