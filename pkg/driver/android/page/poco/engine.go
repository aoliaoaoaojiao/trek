package poco

type Engine string

const (
	Cocos2dxJs   Engine = "COCOS_2DX_JS"
	CocosCreator Engine = "COCOS_CREATOR"
	Egret        Engine = "EGRET"
	Unity3d      Engine = "UNITY_3D"
	Cocos2dxLua  Engine = "COCOS_2DX_LUA"
)

func (e Engine) IsWebSocket() bool {
	return e == Cocos2dxJs || e == CocosCreator || e == Egret
}
