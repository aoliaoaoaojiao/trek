package poco

type Engine string

const (
	Cocos2dxJs    Engine = "COCOS_2DX_JS"
	Cocos2dxCPlus Engine = "COCOS_2DX_C++"
	CocosCreator  Engine = "COCOS_CREATOR"
	Egret         Engine = "EGRET"
	Unity3d       Engine = "UNITY_3D"
	UE4           Engine = "UE4"
	Cocos2dxLua   Engine = "COCOS_2DX_LUA"
)

func (e Engine) GetDefaultPort() int {
	switch e {
	case Unity3d, UE4:
		return 5001
	case Cocos2dxJs, Egret, CocosCreator:
		return 5003
	case Cocos2dxLua:
		return 15004
	case Cocos2dxCPlus:
		return 18888
	}
	return 0
}

func (e Engine) IsWebSocket() bool {
	return e == Cocos2dxJs || e == CocosCreator || e == Egret
}
