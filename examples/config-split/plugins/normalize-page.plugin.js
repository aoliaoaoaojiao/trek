/// <reference path="../../../assets/goja/trek-plugin.d.ts" />

const plugin = {
  transformPage(ctx) {
    const page = ctx.page
    let xml = page.xml || ""

    // 示例：把不稳定的动态文案归一化，降低页面指纹抖动。
    xml = trek.page.patchText(xml, /剩余\d+秒/g, "剩余N秒")
    xml = trek.page.patchText(xml, /验证码\d{4,8}/g, "验证码XXXX")

    return { xml }
  },
}
