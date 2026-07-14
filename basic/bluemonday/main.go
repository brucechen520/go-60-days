package main

import (
	"fmt"
	"strings"

	bluemonday "github.com/trilogy-group/kayako-bluemonday"
)

// 1. 定義系統的「基底白名單陣列」（取代直接使用 UGCPolicy）
var defaultAllowedTags = []string{
	"p", "br", "hr", "b", "i", "strong", "em",
	"ul", "ol", "li", "table", "tr", "td", "th",
	"h1", "h2", "h3",
}

// 輔助函數：從預設名單中，剔除黑名單
func buildAllowedList(baseTags []string, restrictedTags []string) []string {
	// 建立一個 map 來快速查找黑名單，提高效能
	restrictedMap := make(map[string]bool)
	for _, t := range restrictedTags {
		restrictedMap[strings.ToLower(t)] = true
	}

	var finalTags []string
	for _, t := range baseTags {
		if !restrictedMap[strings.ToLower(t)] {
			finalTags = append(finalTags, t)
		}
	}
	return finalTags
}

func main() {

	p := bluemonday.StrictPolicy()
	html := "<p>這是一張<br><b>高優先級</b>的工單<script>alert('XSS');</script><br></p>"

	safeText := p.Sanitize(html)
	// 輸出: "這是一張高優先級的工單"
	fmt.Println(safeText)

	p = bluemonday.UGCPolicy()
	html = `<a href="javascript:alert('XSS')">點擊我</a> <b onclick="attack()"><bad>粗體</bad></b>`
	p.SkipElementsContent("bad")
	safeHTML := p.Sanitize(html)
	// 輸出: "點擊我 粗體"
	fmt.Println(safeHTML)

	rawHTML := `
<p>這是一般文字，可以包含 <b>粗體</b>。</p>
<h1>這是一個會破壞版面的超大標題</h1>
<table><tr><td>我是一筆資料</td></tr></table>
<script>alert("XSS");</script>
`

	// 2. 前端傳來的黑名單規則
	restrictedTags := []string{"h1", "table"}

	// 3. 從零建立一個空白 Policy
	policy := bluemonday.NewPolicy()

	// 4. 計算出「扣除黑名單後，真正該放行的白名單」，並餵給 Policy
	finalAllowed := buildAllowedList(defaultAllowedTags, restrictedTags)
	policy.AllowElements(finalAllowed...)

	// 5. 處理危險標籤（不在白名單內的標籤）的內容
	// 為了避免被剝除的 h1 或 table 留下純文字，我們把它們也加入 SkipElementsContent
	// 加上預設本來就要超渡的 script, style 等
	skipContentTags := append(restrictedTags, "script", "style", "iframe", "object")
	policy.SkipElementsContent(skipContentTags...)

	// 6. 執行過濾
	cleanHTML := policy.Sanitize(rawHTML)

	fmt.Println("====== 淨化後的結果 ======")
	fmt.Println("cleanHTML", cleanHTML)

	// 這是真實的電子郵件內容
	rawHTML = "<div dir=\"ltr\">芙莉蓮熊福利<div><img src=\"cid:ii_mnq5xhbg12\" alt=\"image.png\" width=\"495\" height=\"278\"><br></div></div><br><div class=\"gmail_quote gmail_quote_container\"><div dir=\"ltr\" class=\"gmail_attr\">On Wed, Apr 8, 2026 at 10:46 PM Chen Bruce &lt;<a href=\"mailto:chen.bruce.723@gmail.com\">chen.bruce.723@gmail.com</a>&gt; wrote:<br></div><blockquote class=\"gmail_quote\" style=\"margin:0px 0px 0px 0.8ex;border-left:1px solid rgb(204,204,204);padding-left:1ex\"><div dir=\"ltr\">test<div><img src=\"cid:ii_mnq5vij211\" alt=\"image.png\" width=\"495\" height=\"278\"><br></div></div><br><div class=\"gmail_quote\"><div dir=\"ltr\" class=\"gmail_attr\">On Wed, Apr 8, 2026 at 10:40 PM Chen Bruce &lt;<a href=\"mailto:chen.bruce.723@gmail.com\" target=\"_blank\">chen.bruce.723@gmail.com</a>&gt; wrote:<br></div><blockquote class=\"gmail_quote\" style=\"margin:0px 0px 0px 0.8ex;border-left:1px solid rgb(204,204,204);padding-left:1ex\"><div dir=\"ltr\">收信囉<div><img src=\"cid:ii_mnq5n73w10\" alt=\"image.png\" width=\"495\" height=\"278\"><br></div></div><br><div class=\"gmail_quote\"><div dir=\"ltr\" class=\"gmail_attr\">On Wed, Apr 8, 2026 at 10:38 PM Chen Bruce &lt;<a href=\"mailto:chen.bruce.723@gmail.com\" target=\"_blank\">chen.bruce.723@gmail.com</a>&gt; wrote:<br></div><blockquote class=\"gmail_quote\" style=\"margin:0px 0px 0px 0.8ex;border-left:1px solid rgb(204,204,204);padding-left:1ex\"><div dir=\"ltr\"><div>Carbon六六六<br></div><img src=\"cid:ii_mnq5kcs09\" alt=\"image.png\" width=\"495\" height=\"278\"><br></div><br><div class=\"gmail_quote\"><div dir=\"ltr\" class=\"gmail_attr\">On Wed, Apr 8, 2026 at 10:24 PM Chen Bruce &lt;<a href=\"mailto:chen.bruce.723@gmail.com\" target=\"_blank\">chen.bruce.723@gmail.com</a>&gt; wrote:<br></div><blockquote class=\"gmail_quote\" style=\"margin:0px 0px 0px 0.8ex;border-left:1px solid rgb(204,204,204);padding-left:1ex\"><div dir=\"ltr\">Carbon六六六<div><img src=\"cid:ii_mnq53pck8\" alt=\"image.png\" width=\"495\" height=\"278\"><br></div></div><br><div class=\"gmail_quote\"><div dir=\"ltr\" class=\"gmail_attr\">On Wed, Apr 8, 2026 at 6:48 PM Chen Bruce &lt;<a href=\"mailto:chen.bruce.723@gmail.com\" target=\"_blank\">chen.bruce.723@gmail.com</a>&gt; wrote:<br></div><blockquote class=\"gmail_quote\" style=\"margin:0px 0px 0px 0.8ex;border-left:1px solid rgb(204,204,204);padding-left:1ex\"><div dir=\"ltr\">凱爾文快喔！不要鬧了喔<div><img src=\"cid:ii_mnpxdkwq7\" alt=\"image.png\" width=\"495\" height=\"278\"><br></div></div><br><div class=\"gmail_quote\"><div dir=\"ltr\" class=\"gmail_attr\">On Wed, Apr 8, 2026 at 6:43 PM Chen Bruce &lt;<a href=\"mailto:chen.bruce.723@gmail.com\" target=\"_blank\">chen.bruce.723@gmail.com</a>&gt; wrote:<br></div><blockquote class=\"gmail_quote\" style=\"margin:0px 0px 0px 0.8ex;border-left:1px solid rgb(204,204,204);padding-left:1ex\"><div dir=\"ltr\">真的有消息嗎？<div><br></div><div><img src=\"cid:ii_mnpx75056\" alt=\"image.png\" width=\"495\" height=\"278\"><br></div></div><br><div class=\"gmail_quote\"><div dir=\"ltr\" class=\"gmail_attr\">On Wed, Apr 8, 2026 at 6:15 PM Chen Bruce &lt;<a href=\"mailto:chen.bruce.723@gmail.com\" target=\"_blank\">chen.bruce.723@gmail.com</a>&gt; wrote:<br></div><blockquote class=\"gmail_quote\" style=\"margin:0px 0px 0px 0.8ex;border-left:1px solid rgb(204,204,204);padding-left:1ex\"><div dir=\"ltr\">有消息了嗎？<div><img src=\"cid:ii_mnpw6c1h5\" alt=\"image.png\" width=\"495\" height=\"278\"><br></div></div><br><div class=\"gmail_quote\"><div dir=\"ltr\" class=\"gmail_attr\">On Wed, Apr 8, 2026 at 5:54 PM Chen Bruce &lt;<a href=\"mailto:chen.bruce.723@gmail.com\" target=\"_blank\">chen.bruce.723@gmail.com</a>&gt; wrote:<br></div><blockquote class=\"gmail_quote\" style=\"margin:0px 0px 0px 0.8ex;border-left:1px solid rgb(204,204,204);padding-left:1ex\"><div dir=\"ltr\">快喔<br><img src=\"cid:ii_mnpvfwz34\" alt=\"image.png\" width=\"495\" height=\"278\"><br></div><br><div class=\"gmail_quote\"><div dir=\"ltr\" class=\"gmail_attr\">On Wed, Apr 8, 2026 at 5:47 PM Chen Bruce &lt;<a href=\"mailto:chen.bruce.723@gmail.com\" target=\"_blank\">chen.bruce.723@gmail.com</a>&gt; wrote:<br></div><blockquote class=\"gmail_quote\" style=\"margin:0px 0px 0px 0.8ex;border-left:1px solid rgb(204,204,204);padding-left:1ex\"><div dir=\"ltr\">想請問這件事情有下文了嗎？ 有的話希望盡快通知我們<br><div><br></div><div>感謝您</div><div><br></div><div><img src=\"cid:ii_mnpv6tn83\" alt=\"image.png\" width=\"495\" height=\"278\"><br></div></div><br><div class=\"gmail_quote\"><div dir=\"ltr\" class=\"gmail_attr\">On Wed, Apr 8, 2026 at 5:43 PM Chen Bruce &lt;<a href=\"mailto:chen.bruce.723@gmail.com\" target=\"_blank\">chen.bruce.723@gmail.com</a>&gt; wrote:<br></div><blockquote class=\"gmail_quote\" style=\"margin:0px 0px 0px 0.8ex;border-left:1px solid rgb(204,204,204);padding-left:1ex\"><div dir=\"ltr\">想請問這件事情有下文了嗎？<br><br><img src=\"cid:ii_mnpv153z2\" alt=\"image.png\" width=\"495\" height=\"278\"><br></div><br><div class=\"gmail_quote\"><div dir=\"ltr\" class=\"gmail_attr\">On Tue, Apr 7, 2026 at 9:18 PM Chen Bruce &lt;<a href=\"mailto:chen.bruce.723@gmail.com\" target=\"_blank\">chen.bruce.723@gmail.com</a>&gt; wrote:<br></div><blockquote class=\"gmail_quote\" style=\"margin:0px 0px 0px 0.8ex;border-left:1px solid rgb(204,204,204);padding-left:1ex\"><div dir=\"ltr\">對，我要問的是那個<div><img src=\"cid:ii_mnonac371\" alt=\"image.png\" width=\"495\" height=\"278\"><br></div></div><br><div class=\"gmail_quote\"><div dir=\"ltr\" class=\"gmail_attr\">On Tue, Apr 7, 2026 at 9:17 PM 陳偉男 &lt;<a href=\"mailto:gibe329apt978@gmail.com\" target=\"_blank\">gibe329apt978@gmail.com</a>&gt; wrote:<br></div><blockquote class=\"gmail_quote\" style=\"margin:0px 0px 0px 0.8ex;border-left:1px solid rgb(204,204,204);padding-left:1ex\"><div dir=\"ltr\">您好，想請問你要問的是 kino主機嗎？</div><br><div class=\"gmail_quote\"><div dir=\"ltr\" class=\"gmail_attr\">Chen Bruce &lt;<a href=\"mailto:chen.bruce.723@gmail.com\" target=\"_blank\">chen.bruce.723@gmail.com</a>&gt; 於 2026年4月7日週二 下午9:14寫道：<br></div><blockquote class=\"gmail_quote\" style=\"margin:0px 0px 0px 0.8ex;border-left:1px solid rgb(204,204,204);padding-left:1ex\"><div dir=\"ltr\">你好，想請問主機一些問題<div><br></div><div><img src=\"cid:ii_mnon501p0\" alt=\"image.png\" width=\"475\" height=\"267\"><br></div></div>\r\n</blockquote></div>\r\n</blockquote></div>\r\n</blockquote></div>\r\n</blockquote></div>\r\n</blockquote></div>\r\n</blockquote></div>\r\n</blockquote></div>\r\n</blockquote></div>\r\n</blockquote></div>\r\n</blockquote></div>\r\n</blockquote></div>\r\n</blockquote></div>\r\n</blockquote></div>\r\n<br>"
	s := bluemonday.StripTagsPolicy()
	// s := bluemonday.StrictPolicy()
	a := s.Sanitize(rawHTML)
	fmt.Println("====== StripTagsPolicy ======")
	fmt.Println(a)
	safeText = p.Sanitize(rawHTML)
	// 輸出: "芙莉蓮熊福利"
	// fmt.Println(safeText)
}
