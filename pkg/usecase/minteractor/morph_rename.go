// 指示: miu200521358
package minteractor

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mlib_go/pkg/domain/model/collection"
	"github.com/miu200521358/mlib_go/pkg/shared/base/logging"
)

const (
	morphRenameProgressChunkSize = 25
	morphRenameLogChunk          = 25
	morphRenameTempPrefix        = "__mu_vrm2pmx_morph_tmp_"

	morphRenameInfoStartFormat    = "モーフ名称変換開始(Info): targets=%d mappings=%d"
	morphRenameInfoProgressFormat = "モーフ名称変換: processed=%d/%d renamed=%d unchanged=%d"
	morphRenameInfoDoneFormat     = "モーフ名称変換完了: processed=%d renamed=%d unchanged=%d notFound=%d"
)

// morphRenameRule はrename-only移植用のモーフ対応を表す。
type morphRenameRule struct {
	Name  string
	Panel model.MorphPanel
}

// morphRenameOperation は1モーフ分のrename-only適用情報を表す。
type morphRenameOperation struct {
	Index       int
	SourceName  string
	TargetName  string
	TargetPanel model.MorphPanel
}

// morphRenameSummary はrename-only適用結果の集計を表す。
type morphRenameSummary struct {
	Targets   int
	Mappings  int
	Processed int
	Renamed   int
	Unchanged int
	NotFound  int
}

// morphRenameApplyResult は1モーフ適用結果の詳細を表す。
type morphRenameApplyResult struct {
	NameRenamed        bool
	PanelChanged       bool
	EnglishNameChanged bool
	RenameError        error
	Status             string
}

// morphRenameOnlyRules は名称変更のみで移植可能な表情モーフ対応表を表す。
var morphRenameOnlyRules = map[string]morphRenameRule{
	"Fcl_BRW_Fun_R":            {Name: "にこり右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"Fcl_BRW_Fun_L":            {Name: "にこり左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"Fcl_BRW_Fun":              {Name: "にこり", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"Fcl_BRW_Joy_R":            {Name: "にこり2右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"Fcl_BRW_Joy_L":            {Name: "にこり2左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"Fcl_BRW_Joy":              {Name: "にこり2", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"Fcl_BRW_Sorrow_R":         {Name: "困る右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"Fcl_BRW_Sorrow_L":         {Name: "困る左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"Fcl_BRW_Sorrow":           {Name: "困る", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"Fcl_BRW_Angry_R":          {Name: "怒り右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"Fcl_BRW_Angry_L":          {Name: "怒り左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"Fcl_BRW_Angry":            {Name: "怒り", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"Fcl_BRW_Surprised_R":      {Name: "驚き右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"Fcl_BRW_Surprised_L":      {Name: "驚き左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"Fcl_BRW_Surprised":        {Name: "驚き", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Below_R":             {Name: "下右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Below_L":             {Name: "下左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Below":               {Name: "下", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Abobe_R":             {Name: "上右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Abobe_L":             {Name: "上左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Abobe":               {Name: "上", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Left_R":              {Name: "右眉左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Left_L":              {Name: "左眉左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Left":                {Name: "眉左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Right_R":             {Name: "右眉右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Right_L":             {Name: "左眉右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Right":               {Name: "眉右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Front_R":             {Name: "右眉手前", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Front_L":             {Name: "左眉手前", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Front":               {Name: "眉手前", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Serious_R":           {Name: "真面目右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Serious_L":           {Name: "真面目左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Serious":             {Name: "真面目", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Frown_R":             {Name: "ひそめ右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Frown_L":             {Name: "ひそめ左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"brow_Frown":               {Name: "ひそめ", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"browInnerUp_R":            {Name: "ひそめる2右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"browInnerUp_L":            {Name: "ひそめる2左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"browInnerUp":              {Name: "ひそめる2", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"browDownRight":            {Name: "真面目2右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"browDownLeft":             {Name: "真面目2左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"browDown":                 {Name: "真面目2", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"browOuterUpRight":         {Name: "はんっ右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"browOuterUpLeft":          {Name: "はんっ左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"browOuter":                {Name: "はんっ", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT},
	"Fcl_EYE_Surprised_R":      {Name: "びっくり右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Surprised_L":      {Name: "びっくり左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Surprised":        {Name: "びっくり", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eye_Small_R":              {Name: "瞳小右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eye_Small_L":              {Name: "瞳小左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eye_Small":                {Name: "瞳小", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eye_Big_R":                {Name: "瞳大右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eye_Big_L":                {Name: "瞳大左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eye_Big":                  {Name: "瞳大", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Close_R":          {Name: "ｳｨﾝｸ２右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Close_R_Bone":     {Name: "ｳｨﾝｸ２右ボーン", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_EYE_Close_R_Group":    {Name: "ｳｨﾝｸ２右連動", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Close_L":          {Name: "ウィンク２", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Close_L_Bone":     {Name: "ウィンク２ボーン", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_EYE_Close_L_Group":    {Name: "ウィンク２連動", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Close":            {Name: "まばたき", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Close_Group":      {Name: "まばたき連動", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Joy_R":            {Name: "ウィンク右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Joy_R_Bone":       {Name: "ウィンク右ボーン", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_EYE_Joy_R_Group":      {Name: "ウィンク右連動", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Joy_L":            {Name: "ウィンク", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Joy_L_Bone":       {Name: "ウィンクボーン", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_EYE_Joy_L_Group":      {Name: "ウィンク連動", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Joy":              {Name: "笑い", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Joy_Group":        {Name: "笑い連動", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Fun_R":            {Name: "目を細める右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Fun_L":            {Name: "目を細める左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Fun":              {Name: "目を細める", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"raiseEyelid_R":            {Name: "下瞼上げ右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"raiseEyelid_L":            {Name: "下瞼上げ左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"raiseEyelid":              {Name: "下瞼上げ", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eyeSquintRight":           {Name: "にんまり右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eyeSquintLeft":            {Name: "にんまり左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eyeSquint":                {Name: "にんまり", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Angry_R":          {Name: "ｷﾘｯ右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Angry_L":          {Name: "ｷﾘｯ左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Angry":            {Name: "ｷﾘｯ", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"noseSneerRight":           {Name: "ｷﾘｯ2右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"noseSneerLeft":            {Name: "ｷﾘｯ2左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"noseSneer":                {Name: "ｷﾘｯ2", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Sorrow_R":         {Name: "じと目右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Sorrow_L":         {Name: "じと目左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Sorrow":           {Name: "じと目", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Spread_R":         {Name: "上瞼↑右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Spread_L":         {Name: "上瞼↑左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Spread":           {Name: "上瞼↑", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eye_Nanu_R":               {Name: "なぬ！右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eye_Nanu_L":               {Name: "なぬ！左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eye_Nanu":                 {Name: "なぬ！", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eye_Hide_Vertex":          {Name: "目隠し頂点", Panel: model.MORPH_PANEL_SYSTEM},
	"eye_Hau_Material":         {Name: "はぅ材質", Panel: model.MORPH_PANEL_SYSTEM},
	"eye_Hau":                  {Name: "はぅ", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eye_Hachume_Material":     {Name: "はちゅ目材質", Panel: model.MORPH_PANEL_SYSTEM},
	"eye_Hachume":              {Name: "はちゅ目", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eye_Nagomi_Material":      {Name: "なごみ材質", Panel: model.MORPH_PANEL_SYSTEM},
	"eye_Nagomi":               {Name: "なごみ", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eye_Star_Material":        {Name: "星目材質", Panel: model.MORPH_PANEL_SYSTEM},
	"eye_Heart_Material":       {Name: "はぁと材質", Panel: model.MORPH_PANEL_SYSTEM},
	"eye_Star":                 {Name: "星目", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eye_Heart":                {Name: "はぁと", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Natural":          {Name: "ナチュラル", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eyeWideRight":             {Name: "びっくり2右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eyeWideLeft":              {Name: "びっくり2左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eyeWide":                  {Name: "びっくり2", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eyeLookUpRight":           {Name: "目上右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eyeLookUpLeft":            {Name: "目上左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eyeLookUp":                {Name: "目上", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eyeLookDownRight":         {Name: "目下右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eyeLookDownLeft":          {Name: "目下左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eyeLookDown":              {Name: "目下", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eyeLookInRight":           {Name: "目頭広右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eyeLookInLeft":            {Name: "目頭広左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eyeLookIn":                {Name: "目頭広", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eyeLookOutLeft":           {Name: "目尻広右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eyeLookOutRight":          {Name: "目尻広左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"eyeLookOut":               {Name: "目尻広", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"_eyeIrisMoveBack_R":       {Name: "瞳小2右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"_eyeIrisMoveBack_L":       {Name: "瞳小2左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"_eyeIrisMoveBack":         {Name: "瞳小2", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"_eyeSquint+LowerUp_R":     {Name: "下瞼上げ2右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"_eyeSquint+LowerUp_L":     {Name: "下瞼上げ2左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"_eyeSquint+LowerUp":       {Name: "下瞼上げ2", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Iris_Hide":        {Name: "白目", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Iris_Hide_R":      {Name: "白目右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Iris_Hide_L":      {Name: "白目左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Highlight_Hide":   {Name: "ハイライトなし", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Highlight_Hide_R": {Name: "ハイライトなし右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_EYE_Highlight_Hide_L": {Name: "ハイライトなし左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"Fcl_MTH_A":                {Name: "あ頂点", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_MTH_A_Bone":           {Name: "あボーン", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_MTH_A_Group":          {Name: "あ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_I":                {Name: "い頂点", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_MTH_I_Bone":           {Name: "いボーン", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_MTH_I_Group":          {Name: "い", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_U":                {Name: "う頂点", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_MTH_U_Bone":           {Name: "うボーン", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_MTH_U_Group":          {Name: "う", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_E":                {Name: "え頂点", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_MTH_E_Bone":           {Name: "えボーン", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_MTH_E_Group":          {Name: "え", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_O":                {Name: "お頂点", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_MTH_O_Bone":           {Name: "おボーン", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_MTH_O_Group":          {Name: "お", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Neutral":          {Name: "ん", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Close":            {Name: "一文字", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Up":               {Name: "口上", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Down":             {Name: "口下", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Angry_R":          {Name: "Λ右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Angry_L":          {Name: "Λ左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Angry":            {Name: "Λ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Sage_R":           {Name: "口角下げ右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Sage_L":           {Name: "口角下げ左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Sage":             {Name: "口角下げ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Small":            {Name: "うー", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Large":            {Name: "口横広げ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Fun_R":            {Name: "にっこり右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Fun_L":            {Name: "にっこり左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Fun":              {Name: "にっこり", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Niko_R":           {Name: "にこ右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Niko_L":           {Name: "にこ左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Niko":             {Name: "にこ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Joy":              {Name: "ワ頂点", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_MTH_Joy_Bone":         {Name: "ワボーン", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_MTH_Joy_Group":        {Name: "ワ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Sorrow":           {Name: "▲頂点", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_MTH_Sorrow_Bone":      {Name: "▲ボーン", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_MTH_Sorrow_Group":     {Name: "▲", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_Surprised":        {Name: "わー頂点", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_MTH_Surprised_Bone":   {Name: "わーボーン", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_MTH_Surprised_Group":  {Name: "わー", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_tongueOut":        {Name: "べーボーン", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_MTH_tongueOut_Group":  {Name: "べー", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_tongueUp":         {Name: "ぺろりボーン", Panel: model.MORPH_PANEL_SYSTEM},
	"Fcl_MTH_tongueUp_Group":   {Name: "ぺろり", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"jawOpen":                  {Name: "あああ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"jawForward":               {Name: "顎前", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"jawLeft":                  {Name: "顎左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"jawRight":                 {Name: "顎右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthFunnel":              {Name: "んむー", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthPucker":              {Name: "うう", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthLeft":                {Name: "口左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthRight":               {Name: "口右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthRollUpper":           {Name: "上唇んむー", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthRollLower":           {Name: "下唇んむー", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthRoll":                {Name: "んむー", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthShrugUpper":          {Name: "上唇むむ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthShrugLower":          {Name: "下唇むむ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthShrug":               {Name: "むむ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthDimpleRight":         {Name: "口幅広右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthDimpleLeft":          {Name: "口幅広左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthDimple":              {Name: "口幅広", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthPressRight":          {Name: "薄笑い右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthPressLeft":           {Name: "薄笑い左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthPress":               {Name: "薄笑い", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthSmileRight":          {Name: "にやり2右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthSmileLeft":           {Name: "にやり2左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthSmile":               {Name: "にやり2", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthUpperUpRight":        {Name: "にひ右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthUpperUpLeft":         {Name: "にひ左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthUpperUp":             {Name: "にひ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"cheekSquintRight":         {Name: "にひひ右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"cheekSquintLeft":          {Name: "にひひ左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"cheekSquint":              {Name: "にひひ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthFrownRight":          {Name: "ちっ右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthFrownLeft":           {Name: "ちっ左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthFrown":               {Name: "ちっ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthLowerDownRight":      {Name: "むっ右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthLowerDownLeft":       {Name: "むっ左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthLowerDown":           {Name: "むっ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthStretchRight":        {Name: "ぎりっ右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthStretchLeft":         {Name: "ぎりっ左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"mouthStretch":             {Name: "ぎりっ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"tongueOut":                {Name: "べー", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"_mouthFunnel+SharpenLips": {Name: "うほっ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"_mouthPress+CatMouth":     {Name: "ω口", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"_mouthPress+CatMouth-ex":  {Name: "ω口2", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"_mouthPress+DuckMouth":    {Name: "ω口3", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"cheekPuff_R":              {Name: "ぷくー右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"cheekPuff_L":              {Name: "ぷくー左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"cheekPuff":                {Name: "ぷくー", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_SkinFung_L":       {Name: "肌牙左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_SkinFung_R":       {Name: "肌牙右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_MTH_SkinFung":         {Name: "肌牙", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_HA_Fung1":             {Name: "牙", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_HA_Fung1_Up_R":        {Name: "牙上右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_HA_Fung1_Up_L":        {Name: "牙上左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_HA_Fung1_Up":          {Name: "牙上", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_HA_Fung1_Low_R":       {Name: "牙下右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_HA_Fung1_Low_L":       {Name: "牙下左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_HA_Fung1_Low":         {Name: "牙下", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_HA_Fung2_Up":          {Name: "ギザ歯上", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_HA_Fung2_Low":         {Name: "ギザ歯下", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_HA_Fung2":             {Name: "ギザ歯", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_HA_Fung3_Up":          {Name: "真ん中牙上", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_HA_Fung3_Low":         {Name: "真ん中牙下", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_HA_Fung3":             {Name: "真ん中牙", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_HA_Hide":              {Name: "歯隠", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_HA_Short_Up":          {Name: "歯短上", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_HA_Short_Low":         {Name: "歯短下", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Fcl_HA_Short":             {Name: "歯短", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT},
	"Cheek_Dye":                {Name: "照れ", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT},
	"Fcl_ALL_Neutral":          {Name: "ニュートラル", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT},
	"Fcl_ALL_Angry":            {Name: "怒", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT},
	"Fcl_ALL_Fun":              {Name: "楽", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT},
	"Fcl_ALL_Joy":              {Name: "喜", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT},
	"Fcl_ALL_Sorrow":           {Name: "哀", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT},
	"Fcl_ALL_Surprised":        {Name: "驚", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT},
	"Edge_Off":                 {Name: "エッジOFF", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT},
	"blink":                    {Name: "まばたき", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"blinkLeft":                {Name: "ウィンク２", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"blinkRight":               {Name: "ｳｨﾝｸ２右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT},
	"aa":                       {Name: "あ頂点", Panel: model.MORPH_PANEL_SYSTEM},
	"ih":                       {Name: "い頂点", Panel: model.MORPH_PANEL_SYSTEM},
	"ou":                       {Name: "う頂点", Panel: model.MORPH_PANEL_SYSTEM},
	"ee":                       {Name: "え頂点", Panel: model.MORPH_PANEL_SYSTEM},
	"oh":                       {Name: "お頂点", Panel: model.MORPH_PANEL_SYSTEM},
	"neutral":                  {Name: "ニュートラル", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT},
	"angry":                    {Name: "怒", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT},
	"relaxed":                  {Name: "楽", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT},
	"happy":                    {Name: "喜", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT},
	"sad":                      {Name: "哀", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT},
	"surprised":                {Name: "驚", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT},
}

// applyMorphRenameOnlyBeforeViewer はrename-only対応表に基づきモーフ名・パネルを補正する。
func applyMorphRenameOnlyBeforeViewer(modelData *ModelData, progressReporter IPrepareProgressReporter) morphRenameSummary {
	summary := morphRenameSummary{Mappings: len(morphRenameOnlyRules)}
	summary.Targets = resolveMorphRenameTargetCount(modelData)

	reportPrepareProgress(progressReporter, PrepareProgressEvent{
		Type:       PrepareProgressEventTypeMorphRenamePlanned,
		MorphCount: summary.Targets,
	})
	logMorphRenameInfo(morphRenameInfoStartFormat, summary.Targets, summary.Mappings)
	logMorphRenameModelNames(modelData)

	if modelData == nil || modelData.Morphs == nil || summary.Targets == 0 {
		summary.NotFound = summary.Mappings
		reportPrepareProgress(progressReporter, PrepareProgressEvent{Type: PrepareProgressEventTypeMorphRenameCompleted})
		logMorphRenameInfo(
			morphRenameInfoDoneFormat,
			summary.Processed,
			summary.Renamed,
			summary.Unchanged,
			summary.NotFound,
		)
		return summary
	}

	operations, notFound := collectMorphRenameOperations(modelData.Morphs)
	summary.NotFound = notFound
	renamePlanByIndex := buildMorphRenamePlannedFlags(modelData.Morphs, operations)
	tempRenamed := applyMorphTemporaryRenames(modelData.Morphs, operations, renamePlanByIndex)

	processedPending := 0
	for index := 0; index < modelData.Morphs.Len(); index++ {
		summary.Processed++
		processedPending++

		morphData, err := modelData.Morphs.Get(index)
		if err != nil || morphData == nil {
			summary.Unchanged++
			logMorphRenameDebug(
				"モーフ名称変換詳細: index=%d mapped=%t status=%s",
				index,
				false,
				"morph_nil_or_unreadable",
			)
			processedPending = flushMorphRenameProgress(progressReporter, summary, processedPending, false)
			continue
		}

		sourceName := strings.TrimSpace(morphData.Name())
		beforePanel := morphData.Panel
		targetName := ""
		targetPanel := beforePanel
		renamePlanned := false
		tempName := ""
		applyResult := morphRenameApplyResult{Status: "no_mapping"}
		changed := false
		if operation, exists := operations[index]; exists {
			sourceName = operation.SourceName
			targetName = operation.TargetName
			targetPanel = operation.TargetPanel
			renamePlanned = renamePlanByIndex[index]
			tempName = tempRenamed[index]
			applyResult = applyMorphRenameOperation(
				modelData.Morphs,
				morphData,
				operation,
				renamePlanned,
				tempName,
			)
			changed = applyResult.NameRenamed || applyResult.PanelChanged || applyResult.EnglishNameChanged
		} else {
			logMorphRenameDebug(
				"モーフ名称変換詳細: index=%d source=%s mapped=%t status=%s",
				index,
				sourceName,
				false,
				"no_mapping",
			)
		}
		if changed {
			summary.Renamed++
		} else {
			summary.Unchanged++
		}
		if targetName != "" {
			logMorphRenameDebug(
				"モーフ名称変換詳細: index=%d source=%s target=%s mapped=%t renamePlanned=%t tempAssigned=%t nameRenamed=%t panelChanged=%t englishChanged=%t beforePanel=%d afterPanel=%d status=%s err=%v",
				index,
				sourceName,
				targetName,
				true,
				renamePlanned,
				tempName != "",
				applyResult.NameRenamed,
				applyResult.PanelChanged,
				applyResult.EnglishNameChanged,
				beforePanel,
				targetPanel,
				applyResult.Status,
				applyResult.RenameError,
			)
		}
		processedPending = flushMorphRenameProgress(progressReporter, summary, processedPending, false)
	}
	flushMorphRenameProgress(progressReporter, summary, processedPending, true)

	reportPrepareProgress(progressReporter, PrepareProgressEvent{Type: PrepareProgressEventTypeMorphRenameCompleted})
	logMorphRenameInfo(
		morphRenameInfoDoneFormat,
		summary.Processed,
		summary.Renamed,
		summary.Unchanged,
		summary.NotFound,
	)
	return summary
}

// logMorphRenameModelNames はモデルに存在するモーフ名一覧をDEBUGログ出力する。
func logMorphRenameModelNames(modelData *ModelData) {
	if modelData == nil || modelData.Morphs == nil {
		logMorphRenameDebug("モーフ名称一覧: count=0 status=model_or_morphs_nil")
		return
	}

	count := modelData.Morphs.Len()
	logMorphRenameDebug("モーフ名称一覧開始: count=%d", count)
	for index := 0; index < count; index++ {
		morphData, err := modelData.Morphs.Get(index)
		if err != nil || morphData == nil {
			logMorphRenameDebug("モーフ名称一覧: index=%d status=morph_nil_or_unreadable", index)
			continue
		}
		name := strings.TrimSpace(morphData.Name())
		if name == "" {
			logMorphRenameDebug("モーフ名称一覧: index=%d panel=%d status=name_empty", index, morphData.Panel)
			continue
		}
		logMorphRenameDebug("モーフ名称一覧: index=%d name=%s panel=%d", index, name, morphData.Panel)
	}
	logMorphRenameDebug("モーフ名称一覧終了: count=%d", count)
}

// resolveMorphRenameTargetCount はrename-only対象件数（モーフ総数）を返す。
func resolveMorphRenameTargetCount(modelData *ModelData) int {
	if modelData == nil || modelData.Morphs == nil {
		return 0
	}
	return modelData.Morphs.Len()
}

// collectMorphRenameOperations はモデル内モーフ名と対応表を照合し、操作一覧と未検出件数を返す。
func collectMorphRenameOperations(morphs *collection.NamedCollection[*model.Morph]) (map[int]morphRenameOperation, int) {
	operations := map[int]morphRenameOperation{}
	if morphs == nil {
		return operations, len(morphRenameOnlyRules)
	}

	foundSources := map[string]struct{}{}
	for index := 0; index < morphs.Len(); index++ {
		morphData, err := morphs.Get(index)
		if err != nil || morphData == nil {
			continue
		}
		sourceName := strings.TrimSpace(morphData.Name())
		if sourceName == "" {
			continue
		}
		rule, exists := morphRenameOnlyRules[sourceName]
		if !exists {
			continue
		}
		operations[index] = morphRenameOperation{
			Index:       index,
			SourceName:  sourceName,
			TargetName:  rule.Name,
			TargetPanel: rule.Panel,
		}
		foundSources[sourceName] = struct{}{}
	}
	notFound := len(morphRenameOnlyRules) - len(foundSources)
	if notFound < 0 {
		notFound = 0
	}
	return operations, notFound
}

// buildMorphRenamePlannedFlags は各モーフのrename実行可否を返す。
func buildMorphRenamePlannedFlags(
	morphs *collection.NamedCollection[*model.Morph],
	operations map[int]morphRenameOperation,
) map[int]bool {
	planned := map[int]bool{}
	renameOps := map[int]morphRenameOperation{}
	targets := map[string][]int{}

	for index, operation := range operations {
		if operation.SourceName == operation.TargetName {
			planned[index] = false
			continue
		}
		planned[index] = true
		renameOps[index] = operation
		targets[operation.TargetName] = append(targets[operation.TargetName], index)
	}

	for _, indexes := range targets {
		if len(indexes) < 2 {
			continue
		}
		for _, index := range indexes {
			planned[index] = false
		}
	}

	for index, operation := range renameOps {
		if !planned[index] || morphs == nil {
			continue
		}
		existing, err := morphs.GetByName(operation.TargetName)
		if err != nil || existing == nil {
			continue
		}
		existingIndex := existing.Index()
		if existingIndex == index {
			continue
		}
		if _, moving := renameOps[existingIndex]; moving {
			continue
		}
		planned[index] = false
	}

	return planned
}

// applyMorphTemporaryRenames は衝突回避のためrename対象モーフへ一時名を割り当てる。
func applyMorphTemporaryRenames(
	morphs *collection.NamedCollection[*model.Morph],
	operations map[int]morphRenameOperation,
	planned map[int]bool,
) map[int]string {
	tempRenamed := map[int]string{}
	if morphs == nil {
		return tempRenamed
	}

	indexes := make([]int, 0, len(planned))
	for index, canRename := range planned {
		if !canRename {
			continue
		}
		if _, exists := operations[index]; !exists {
			continue
		}
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)

	serial := 0
	for _, index := range indexes {
		tempName := nextTemporaryMorphName(morphs, &serial)
		renamed, err := morphs.Rename(index, tempName)
		if err != nil || !renamed {
			planned[index] = false
			continue
		}
		tempRenamed[index] = tempName
	}

	return tempRenamed
}

// applyMorphRenameOperation は1モーフ分のrename-only補正を適用し、変更有無を返す。
func applyMorphRenameOperation(
	morphs *collection.NamedCollection[*model.Morph],
	morphData *model.Morph,
	operation morphRenameOperation,
	canRename bool,
	tempName string,
) morphRenameApplyResult {
	result := morphRenameApplyResult{Status: "unchanged"}
	if morphData == nil {
		result.Status = "morph_nil"
		return result
	}

	if canRename {
		if tempName != "" && morphs != nil {
			renamed, err := morphs.Rename(operation.Index, operation.TargetName)
			if err != nil {
				result.RenameError = err
				result.Status = "rename_error"
			} else if renamed {
				result.NameRenamed = true
				result.Status = "name_renamed"
			} else {
				result.Status = "rename_not_applied"
			}
			if refreshed, getErr := morphs.Get(operation.Index); getErr == nil && refreshed != nil {
				morphData = refreshed
			}
		} else {
			result.Status = "rename_skipped_temp_not_assigned"
		}
	} else {
		result.Status = "rename_skipped"
	}

	if morphData.Panel != operation.TargetPanel {
		morphData.Panel = operation.TargetPanel
		result.PanelChanged = true
	}

	if morphData.Name() == operation.TargetName && morphData.EnglishName != operation.TargetName {
		morphData.EnglishName = operation.TargetName
		result.EnglishNameChanged = true
	}

	if result.NameRenamed && result.PanelChanged && result.EnglishNameChanged {
		result.Status = "name_panel_english_updated"
	} else if result.NameRenamed && result.PanelChanged {
		result.Status = "name_panel_updated"
	} else if result.NameRenamed {
		result.Status = "name_updated"
	} else if result.PanelChanged {
		result.Status = "panel_updated_only"
	} else if result.EnglishNameChanged {
		result.Status = "english_updated_only"
	}

	return result
}

// nextTemporaryMorphName は重複しない一時モーフ名を返す。
func nextTemporaryMorphName(morphs *collection.NamedCollection[*model.Morph], serial *int) string {
	if serial == nil {
		return morphRenameTempPrefix + "000"
	}
	for {
		candidate := fmt.Sprintf("%s%03d", morphRenameTempPrefix, *serial)
		*serial = *serial + 1
		if morphs == nil {
			return candidate
		}
		if _, err := morphs.GetByName(candidate); err != nil {
			return candidate
		}
	}
}

// flushMorphRenameProgress は進捗イベント/INFOログをチャンク単位で出力し、未送信件数を返す。
func flushMorphRenameProgress(
	progressReporter IPrepareProgressReporter,
	summary morphRenameSummary,
	pending int,
	force bool,
) int {
	if pending < morphRenameProgressChunkSize && !(force && pending > 0) {
		return pending
	}
	reportPrepareProgress(progressReporter, PrepareProgressEvent{
		Type:       PrepareProgressEventTypeMorphRenameProcessed,
		MorphCount: pending,
	})
	logMorphRenameInfo(
		morphRenameInfoProgressFormat,
		summary.Processed,
		summary.Targets,
		summary.Renamed,
		summary.Unchanged,
	)
	return 0
}

// logMorphRenameInfo はモーフ名称変換のINFOログを出力する。
func logMorphRenameInfo(format string, params ...any) {
	logger := logging.DefaultLogger()
	if logger == nil {
		return
	}
	logger.Info(format, params...)
	if logger.IsVerboseEnabled(logging.VERBOSE_INDEX_VIEWER) {
		logger.Verbose(logging.VERBOSE_INDEX_VIEWER, "[INFO] "+format, params...)
	}
}

// logMorphRenameDebug はモーフ名称変換のDEBUGログを出力する。
func logMorphRenameDebug(format string, params ...any) {
	logger := logging.DefaultLogger()
	if logger == nil {
		return
	}
	logger.Debug(format, params...)
	if logger.IsVerboseEnabled(logging.VERBOSE_INDEX_VIEWER) {
		logger.Verbose(logging.VERBOSE_INDEX_VIEWER, "[DEBUG] "+format, params...)
	}
}

// vrm1ExpressionDefinitionCheck はVRM1表情定義の有無判定用構造を表す。
type vrm1ExpressionDefinitionCheck struct {
	Expressions vrm1ExpressionSetDefinitionCheck `json:"expressions"`
}

// vrm1ExpressionSetDefinitionCheck はVRM1 preset/custom表情定義を表す。
type vrm1ExpressionSetDefinitionCheck struct {
	Preset map[string]json.RawMessage `json:"preset"`
	Custom map[string]json.RawMessage `json:"custom"`
}

// vrm0BlendShapeDefinitionCheck はVRM0表情定義の有無判定用構造を表す。
type vrm0BlendShapeDefinitionCheck struct {
	BlendShapeMaster vrm0BlendShapeMasterDefinitionCheck `json:"blendShapeMaster"`
}

// vrm0BlendShapeMasterDefinitionCheck はVRM0 blendShapeGroups定義を表す。
type vrm0BlendShapeMasterDefinitionCheck struct {
	BlendShapeGroups []json.RawMessage `json:"blendShapeGroups"`
}

// hasVrmExpressionDefinitions はVRM拡張に表情定義があるか判定する。
func hasVrmExpressionDefinitions(modelData *ModelData) bool {
	if modelData == nil || modelData.VrmData == nil || modelData.VrmData.RawExtensions == nil {
		return false
	}
	rawExtensions := modelData.VrmData.RawExtensions
	if raw, exists := rawExtensions["VRMC_vrm"]; exists && hasVrm1ExpressionDefinitions(raw) {
		return true
	}
	if raw, exists := rawExtensions["VRM"]; exists && hasVrm0BlendShapeDefinitions(raw) {
		return true
	}
	return false
}

// hasVrm1ExpressionDefinitions はVRM1 expressions定義の有無を返す。
func hasVrm1ExpressionDefinitions(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	check := vrm1ExpressionDefinitionCheck{}
	if err := json.Unmarshal(raw, &check); err != nil {
		return false
	}
	return len(check.Expressions.Preset) > 0 || len(check.Expressions.Custom) > 0
}

// hasVrm0BlendShapeDefinitions はVRM0 blendShapeGroups定義の有無を返す。
func hasVrm0BlendShapeDefinitions(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	check := vrm0BlendShapeDefinitionCheck{}
	if err := json.Unmarshal(raw, &check); err != nil {
		return false
	}
	return len(check.BlendShapeMaster.BlendShapeGroups) > 0
}
