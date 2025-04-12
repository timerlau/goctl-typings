package generate

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/tools/goctl/api/parser"
	"github.com/zeromicro/go-zero/tools/goctl/api/spec"
	apiutil "github.com/zeromicro/go-zero/tools/goctl/api/util"
	"github.com/zeromicro/go-zero/tools/goctl/plugin"
	"github.com/zeromicro/go-zero/tools/goctl/util"
	"github.com/zeromicro/go-zero/tools/goctl/util/pathx"
)

const (
	packagePrefix = "components."
	pathPrefix    = "pathPrefix"
)

const (
	formTagKey   = "form"
	pathTagKey   = "path"
	headerTagKey = "header"
)

const (
	componentsTemplate = `// Code generated by goctl. DO NOT EDIT.
// goctl {{.version}}

declare namespace API {
{{.componentTypes}}
}

export { API };
`
)

// Do 生成命令
func Do(in *plugin.Plugin, filename string) error {
	api, err := parser.Parse(in.ApiFilePath)
	if err != nil {
		return err
	}
	// 验证 API 定义
	if err := api.Validate(); err != nil {
		return err
	}
	// 拼接插件地址
	filename = in.Dir + "/" + filename
	dir := filepath.Dir(filename)
	filename = filepath.Base(filename)
	logx.Must(pathx.MkdirIfNotExist(dir))

	types := api.Types
	if len(types) == 0 {
		return nil
	}

	val, err := BuildTypes(types)
	if err != nil {
		return err
	}
	if err := pathx.RemoveIfExist(filename); err != nil {
		fmt.Println("remove file error", err)
		return err
	}

	fp, created, err := apiutil.MaybeCreateFile(dir, ".", filename)
	if err != nil {
		fmt.Println("create file error", err)
		return err
	}
	if !created {
		fmt.Println("file already exists")
		return nil
	}
	defer fp.Close()

	t := template.Must(template.New("componentsTemplate").Parse(componentsTemplate))
	return t.Execute(fp, map[string]interface{}{
		"componentTypes": template.HTML(val),
		"version":        in.Api.Info.Properties["version"],
	})
}

func primitiveType(tp string) (string, bool) {
	// 将 uint64 和 int64 转换为 string
	switch tp {
	case "string", "uint64", "int64":
		return "string", true
	case "int", "int8", "int16", "int32", "uint", "uint8", "uint16", "uint32":
		return "number", true
	case "float", "float32", "float64":
		return "number", true
	case "bool":
		return "boolean", true
	case "[]byte":
		return "Blob", true
	case "interface{}", "any":
		return "any", true
	}
	return "", false
}

func BuildTypes(types []spec.Type) (string, error) {
	var builder strings.Builder
	first := true
	for _, tp := range types {
		if first {
			first = false
		} else {
			builder.WriteString("\n")
		}
		if err := writeType(&builder, tp); err != nil {
			return "", fmt.Errorf("Type %s generate error: %w", tp.Name(), err)
		}
	}

	return builder.String(), nil
}

func writeType(writer io.Writer, tp spec.Type) error {
	if docs := tp.Documents(); len(docs) > 0 {
		writeIndent(writer, 1)
		fmt.Fprintf(writer, "%s\n", strings.Join(docs, "\n"))
	}
	writeIndent(writer, 1)
	fmt.Fprintf(writer, "export interface %s {\n", util.Title(tp.Name()))
	if err := writeMembers(writer, tp, false, 2); err != nil {
		return err
	}

	// 为所有接口添加索引签名，使类型更灵活
	writeIndent(writer, 2)
	// fmt.Fprintf(writer, "[key: string]: any;\n")
	writeIndent(writer, 1)
	fmt.Fprintf(writer, "}\n\n")
	return genParamsTypesIfNeed(writer, tp)
}

func writeMembers(writer io.Writer, tp spec.Type, isParam bool, indent int) error {
	definedType, ok := tp.(spec.DefineStruct)
	if !ok {
		pointType, ok := tp.(spec.PointerType)
		if ok {
			return writeMembers(writer, pointType.Type, isParam, indent)
		}

		return fmt.Errorf("type %s not supported", tp.Name())
	}

	members := definedType.GetBodyMembers()
	if isParam {
		members = definedType.GetNonBodyMembers()
	}
	for _, member := range members {
		if member.IsInline {
			if err := writeMembers(writer, member.Type, isParam, indent); err != nil {
				return err
			}
			continue
		}

		if err := writeProperty(writer, member, indent); err != nil {
			return fmt.Errorf("Type %s generate error: %w", tp.Name(), err)
		}
	}
	return nil
}

func genParamsTypesIfNeed(writer io.Writer, tp spec.Type) error {
	definedType, ok := tp.(spec.DefineStruct)
	if !ok {
		return errors.New("no members of type " + tp.Name())
	}

	members := definedType.GetNonBodyMembers()
	if len(members) == 0 {
		return nil
	}

	writeIndent(writer, 1)
	fmt.Fprintf(writer, "export interface %sParams {\n", util.Title(tp.Name()))
	if err := writeTagMembers(writer, tp, formTagKey); err != nil {
		return err
	}
	writeIndent(writer, 1)
	fmt.Fprintf(writer, "}\n\n")

	if len(definedType.GetTagMembers(headerTagKey)) > 0 {
		writeIndent(writer, 1)
		fmt.Fprintf(writer, "export interface %sHeaders {\n", util.Title(tp.Name()))
		if err := writeTagMembers(writer, tp, headerTagKey); err != nil {
			return err
		}
		writeIndent(writer, 1)
		fmt.Fprintf(writer, "}\n\n")
	}

	return nil
}

func writeProperty(writer io.Writer, member spec.Member, indent int) error {
	writeIndent(writer, indent)
	ty, err := genTsType(member, indent)
	if err != nil {
		return err
	}

	// 所有字段都设置为可选
	optionalTag := "?"

	name, err := member.GetPropertyName()
	if err != nil {
		return err
	}

	comment := member.GetComment()
	if len(comment) > 0 {
		comment = strings.TrimPrefix(comment, "//")
		comment = " // " + strings.TrimSpace(comment)
	}
	if len(member.Docs) > 0 {
		fmt.Fprintf(writer, "%s\n", strings.Join(member.Docs, ""))
		writeIndent(writer, indent)
	}
	_, err = fmt.Fprintf(writer, "%s%s: %s%s\n", name, optionalTag, ty, comment)
	return err
}

func writeIndent(writer io.Writer, indent int) {
	for i := 0; i < indent; i++ {
		fmt.Fprint(writer, "\t")
	}
}

func genTsType(m spec.Member, indent int) (ty string, err error) {
	v, ok := m.Type.(spec.NestedStruct)
	if ok {
		writer := bytes.NewBuffer(nil)
		_, err := fmt.Fprintf(writer, "{\n")
		if err != nil {
			return "", err
		}

		if err := writeMembers(writer, v, false, indent+1); err != nil {
			return "", err
		}

		writeIndent(writer, indent)
		_, err = fmt.Fprintf(writer, "}")
		if err != nil {
			return "", err
		}
		return writer.String(), nil
	}

	ty, err = goTypeToTs(m.Type, false)
	if enums := m.GetEnumOptions(); enums != nil {
		if ty == "string" {
			for i := range enums {
				enums[i] = "'" + enums[i] + "'"
			}
		}
		ty = strings.Join(enums, " | ")
	}
	return
}

func goTypeToTs(tp spec.Type, fromPacket bool) (string, error) {
	switch v := tp.(type) {
	case spec.DefineStruct:
		return addPrefix(tp, fromPacket), nil
	case spec.PrimitiveType:
		r, ok := primitiveType(tp.Name())
		if !ok {
			return "", errors.New("unsupported primitive type " + tp.Name())
		}

		return r, nil
	case spec.MapType:
		valueType, err := goTypeToTs(v.Value, fromPacket)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("{ [key: string]: %s }", valueType), nil
	case spec.ArrayType:
		if tp.Name() == "[]byte" {
			return "Blob", nil
		}

		valueType, err := goTypeToTs(v.Value, fromPacket)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Array<%s>", valueType), nil
	case spec.InterfaceType:
		return "any", nil
	case spec.PointerType:
		return goTypeToTs(v.Type, fromPacket)
	}

	return "", errors.New("unsupported type " + tp.Name())
}

func addPrefix(tp spec.Type, fromPacket bool) string {
	if fromPacket {
		return packagePrefix + util.Title(tp.Name())
	}
	return util.Title(tp.Name())
}

func writeTagMembers(writer io.Writer, tp spec.Type, tagKey string) error {
	definedType, ok := tp.(spec.DefineStruct)
	if !ok {
		pointType, ok := tp.(spec.PointerType)
		if ok {
			return writeTagMembers(writer, pointType.Type, tagKey)
		}

		return fmt.Errorf("type %s not supported", tp.Name())
	}

	members := definedType.GetTagMembers(tagKey)
	for _, member := range members {
		if member.IsInline {
			if err := writeTagMembers(writer, member.Type, tagKey); err != nil {
				return err
			}
			continue
		}

		if err := writeProperty(writer, member, 1); err != nil {
			return fmt.Errorf("Type %s generate error: %w", tp.Name(), err)
		}
	}
	return nil
}
