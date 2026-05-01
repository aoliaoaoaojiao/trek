export type ConfigPayload = {
  page_source: "uia" | "poco"
  page_name_strategy: PageNameStrategy
  touch_mode: "motion" | "uia" | "adb"
  skip_all_actions_from_model: boolean
  uia: { server_port: number }
  poco: { engine: string; port: number }
  log: { file_level: string }
  effective_touch_area: {
    serial: string
    package_name: string
    range: EffectiveRange
  }
}

export type PartialConfigPayload = Partial<{
  page_source: "uia" | "poco"
  page_name_strategy: PageNameStrategy
  touch_mode: "motion" | "uia" | "adb"
  skip_all_actions_from_model: boolean
  uia: Partial<{ server_port: number }>
  poco: Partial<{ engine: string; port: number }>
  log: Partial<{ file_level: string }>
  effective_touch_area: Partial<{
    serial: string
    package_name: string
    range: Partial<EffectiveRange>
  }>
}>

export type PageNameStrategy =
  | ""
  | "uia_activity_first"
  | "xml_only"
  | "xml_fingerprint"
  | "structure_fingerprint"
  | "activity_only"
  | "image_fingerprint"

export type DeviceOption = {
  serial: string
  label: string
}

export type BoundsRect = {
  left: number
  top: number
  right: number
  bottom: number
}

export type DumpTreeNode = {
  id: string
  tag: string
  attrs: Record<string, string>
  bounds: BoundsRect | null
  children: DumpTreeNode[]
}

export type ClickPoint = {
  imagePercentX: number
  imagePercentY: number
  percentX: number
  percentY: number
  absoluteX: number
  absoluteY: number
}

export type EffectiveRange = {
  left: number
  top: number
  right: number
  bottom: number
}

export type ActionType = "click" | "scroll" | "long_press" | "custom_touch"

export type PageActionRule = {
  page_name: string
  action_type: ActionType
  path?: string
  point?: { x: number; y: number }
  start?: { x: number; y: number }
  end?: { x: number; y: number }
}
