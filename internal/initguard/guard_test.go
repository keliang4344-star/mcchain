// Package initguard 用**静态源码扫描**守住一类「编译/单测全绿、链却起不来」的
// 初始化顺序缺陷，作为回归防线。
//
// 缺陷形态
//
//	Go 的包级变量初始化表达式在 main() 之前求值。如果某个包级 var 里拿到了一个
//	bech32 地址字符串（直接取、或经由同包 helper 间接取），此时
//	cmd/mcchaind/cmd.initSDKConfig() 尚未执行，全局 bech32 前缀还是 SDK 默认的
//	"cosmos"，于是：
//	  1. 这个 var 自己固化了一个 cosmos 前缀的字符串；
//	  2. sdk.AccAddress.String() 内部的全局 LRU 缓存（types/address.go 的 accAddrCache）
//	     以「原始地址字节」为 key、完全不带前缀维度，且 SetBech32PrefixForAccount
//	     不会失效它 —— 这一次调用把错误结果永久钉住；
//	  3. 此后 app.New() 中 11 处 authtypes.NewModuleAddress(gov).String() 全部返回
//	     cosmos 前缀，bankkeeper.NewBaseKeeper 的 authority 校验 panic，整条链无法启动
//	     （panic: invalid bank authority address: expected mc, got cosmos）。
//
// 为什么必须用静态扫描而不是运行时断言：单元测试进程里测试会自己先设好前缀，
// 或根本不构造完整 App，所以这类缺陷极容易漏网——一旦存活到生产，
// 会让链在 InitChain 阶段直接 panic，且不依赖任何运行期条件。
package initguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// 目录级跳过名单：第三方缓存、构建产物、链数据、版本库元数据。
var skipDirs = map[string]bool{
	".git": true, "build": true, ".devnet": true, "node_modules": true, "vendor": true,
}

// 命名中带 address/addr 的标识符，视为「产出地址」的信号。
// 例：NewModuleAddress / AccAddress / ValAddress / DefaultGovernorAddress / BlackHoleAddress。
var addrNameRe = regexp.MustCompile(`(?i)addr`)

// allowMarker 是人工豁免标记：写在包级 var 的文档注释里即跳过检查。
const allowMarker = "initguard:allow"

func calleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

// addrStringCall 判定：X.String()，且 X 本身是一次「产出地址」的调用。
// 这是真正会污染 SDK 全局地址缓存的那一步。
func addrStringCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "String" {
		return false
	}
	inner, ok := sel.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	return addrNameRe.MatchString(calleeName(inner.Fun))
}

// bodyContainsAddrString 判断函数体里是否直接出现 X.String() 的地址取值。
func bodyContainsAddrString(fn *ast.FuncDecl) bool {
	if fn.Body == nil {
		return false
	}
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if found {
			return false
		}
		if call, ok := n.(*ast.CallExpr); ok && addrStringCall(call) {
			found = true
			return false
		}
		return true
	})
	return found
}

// scanFile 返回该文件中「包级 var 在 init 阶段固化地址字符串」的位置。
//
// pkgAddrHelpers 是**同包**内「函数体内含地址 String() 取值」的函数名集合，
// 用于捕捉间接形态：包级 var 调的 helper 自己不写 .String()，但 helper 里面写了。
// （这正是真实事故的形态 —— 直接形态反而更好发现。）
func scanFile(fset *token.FileSet, file *ast.File, pkgAddrHelpers map[string]bool) []token.Position {
	var out []token.Position

	for _, decl := range file.Decls {
		// 形态 3：包级 var 不是唯一的 init 阶段执行面。
		// `func init()` 同样在 main() 之前跑，在这里取地址字符串会一模一样地
		// 污染 SDK 全局地址缓存。只扫 GenDecl 会漏掉 init。
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "init" && fn.Recv == nil {
			if fn.Body == nil || hasAllowMarkerFunc(fn) {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, callOK := n.(*ast.CallExpr)
				if !callOK {
					return true
				}
				if addrStringCall(call) {
					out = append(out, fset.Position(call.Pos()))
					return true
				}
				if id, isIdent := call.Fun.(*ast.Ident); isIdent && pkgAddrHelpers[id.Name] {
					out = append(out, fset.Position(call.Pos()))
				}
				return true
			})
			continue
		}

		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		if hasAllowMarker(gen) {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, val := range vs.Values {
				ast.Inspect(val, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					// 形态 1（直接）：authtypes.NewModuleAddress("gov").String()
					if addrStringCall(call) {
						out = append(out, fset.Position(call.Pos()))
						return true
					}
					// 形态 2（间接）：DefaultGovernorAddress()，其函数体内有形态 1
					if id, ok := call.Fun.(*ast.Ident); ok && pkgAddrHelpers[id.Name] {
						out = append(out, fset.Position(call.Pos()))
						return true
					}
					return true
				})
			}
		}
	}
	return out
}

func hasAllowMarker(gen *ast.GenDecl) bool {
	// 豁免方式：在包级 var 的文档注释里写上 `initguard:allow` 并说明理由。
	return gen.Doc != nil && strings.Contains(gen.Doc.Text(), allowMarker)
}

// hasAllowMarkerFunc 是 init() 版本的豁免判定（FuncDecl 的注释挂点不同）。
func hasAllowMarkerFunc(fn *ast.FuncDecl) bool {
	return fn.Doc != nil && strings.Contains(fn.Doc.Text(), allowMarker)
}

// collectPkgAddrHelpers 汇总同包内「函数体内含地址 String() 取值」的顶层函数名。
func collectPkgAddrHelpers(fset *token.FileSet, files []*ast.File) map[string]bool {
	helpers := map[string]bool{}
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue // 只看顶层函数，方法不会被包级 var 直接调用
			}
			if bodyContainsAddrString(fn) {
				helpers[fn.Name.Name] = true
			}
		}
	}
	_ = fset
	return helpers
}

// ---------------------------------------------------------------------------
// 测试 1：先证明检测器本身不是空转 —— 已知坏写法必须报，正确写法必须不报。
// ---------------------------------------------------------------------------

func TestDetectorCatchesKnownBadPatterns(t *testing.T) {
	const (
		directBad = `package p

import authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

var DefaultCfg = Cfg{Governor: authtypes.NewModuleAddress("gov").String()}
`
		// 真实事故形态：包级 var 调用同包 helper，helper 内部才取 .String()
		indirectBad = `package p

import authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

var DefaultCfg = Cfg{Governor: DefaultGovernorAddress()}

func DefaultGovernorAddress() string {
	return authtypes.NewModuleAddress("gov").String()
}
`
		good = `package p

var DefaultTimelock uint64 = 43200

// 延迟求值：前缀设置完成之后才计算地址字符串，不污染 SDK 全局地址缓存。
func DefaultCfg() Cfg { return Cfg{Governor: DefaultGovernorAddress()} }

func DefaultGovernorAddress() string { return "mc1abc" }
`
		// 只取字节（不取字符串）是安全的：字节不含前缀维度。
		bytesOnly = `package p

import authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

var GovAddrBytes = authtypes.NewModuleAddress("gov")
`
		// init 与包级 var 一样在 main 之前执行，
		// 在这里取地址字符串同样会污染 SDK 全局地址缓存。
		initBad = `package p

import authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

var Governor string

func init() {
	Governor = authtypes.NewModuleAddress("gov").String()
}
`
		initGood = `package p

import authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

var Governor string

func init() {
	Governor = string(authtypes.NewModuleAddress("gov"))
}
`
	)

	cases := []struct {
		name string
		src  string
		want int
	}{
		{"直接形态（包级 var 内 .String()）→ 必须命中", directBad, 1},
		{"间接形态（包级 var 调 helper）→ 必须命中", indirectBad, 1},
		{"延迟求值到函数内 → 必须不命中", good, 0},
		{"只取地址字节、不取字符串 → 必须不命中", bytesOnly, 0},
		{"init() 内取地址字符串 → 必须命中", initBad, 1},
		{"init() 内只取地址字节 → 必须不命中", initGood, 0},
	}

	for _, tc := range cases {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "sample.go", tc.src, parser.SkipObjectResolution|parser.ParseComments)
		if err != nil {
			t.Fatalf("%s: 解析失败: %v", tc.name, err)
		}
		helpers := collectPkgAddrHelpers(fset, []*ast.File{f})
		got := scanFile(fset, f, helpers)
		if len(got) != tc.want {
			t.Errorf("%s: 命中 %d 处，期望 %d 处", tc.name, len(got), tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// 测试 2：扫描全仓，守住回归。
// ---------------------------------------------------------------------------

func TestNoPackageLevelAddrStringInRepo(t *testing.T) {
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()

	var offenders []string
	scannedFiles := 0

	err := filepath.WalkDir(root, func(dir string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if skipDirs[d.Name()] {
			return fs.SkipDir
		}

		// 逐包处理：先把同包所有非测试文件一起解析，才能做「包级 var → 同包 helper」分析。
		entries, derr := os.ReadDir(dir)
		if derr != nil {
			return derr
		}
		var files []*ast.File
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			f, perr := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution|parser.ParseComments)
			if perr != nil {
				continue // 源码扫描器不是编译器；解析失败交给 go build 报错
			}
			files = append(files, f)
		}
		if len(files) == 0 {
			return nil
		}

		helpers := collectPkgAddrHelpers(fset, files)
		for _, f := range files {
			scannedFiles++
			for _, pos := range scanFile(fset, f, helpers) {
				rel, rerr := filepath.Rel(root, pos.Filename)
				if rerr != nil {
					rel = pos.Filename
				}
				offenders = append(offenders, filepath.ToSlash(rel)+":"+strconv.Itoa(pos.Line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	// 自证扫描确实覆盖了代码库：否则「一个文件都没解析」也会让门禁假装通过。
	if scannedFiles < 100 {
		t.Fatalf("扫描只覆盖了 %d 个 Go 文件，明显偏低——门禁可能因路径问题空转，请检查 root 推导", scannedFiles)
	}

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("检测到 %d 处「包级 var 在 init 阶段固化地址字符串」的写法。\n"+
			"这类写法会在 bech32 前缀被设置之前把 cosmos 前缀钉进 SDK 的全局地址缓存，\n"+
			"导致 app.New 里 bank keeper 的 authority 校验 panic、整条链无法启动。\n"+
			"修法：把该变量改成函数（延迟求值），例如 DefaultGovernanceHandoverConfig()。\n"+
			"命中位置：\n  %s", len(offenders), strings.Join(offenders, "\n  "))
	}
}
