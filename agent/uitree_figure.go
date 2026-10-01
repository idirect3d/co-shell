// Package agent - figure shape whitelist and validation (FEATURE-570).
//
// A figure node is a vector canvas whose children are geometry shapes. Shapes
// are not standalone components: validateUINode accepts them only inside a
// figure subtree. This file owns the shape whitelist and the numeric/scale
// checks that keep an LLM-drawn figure renderable and cheap to paint.
//
// Author: L.Shuang
// Created: 2026-10-01
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/i18n"
)

// Figure shape type names. Shared verbatim with web/static/ui-figure.js.
const (
	UIShapePoint    = "point"
	UIShapeSegment  = "segment"
	UIShapePolygon  = "polygon"
	UIShapeEllipse  = "ellipse"
	UIShapeAxes     = "axes"
	UIShapeLabel    = "label"
	UIShapePath     = "path"
	UIShapeFunction = "function"
)

// Figure limits (kept next to the frontend constants so error text agrees).
const (
	// UIMaxShapePoints is the maximum number of points of one polygon/path.
	UIMaxShapePoints = 400
	// UIMaxExprLength is the maximum length of one function expression.
	UIMaxExprLength = 500
)

// uiShapeTypes is the whitelist of figure shapes.
var uiShapeTypes = []string{
	UIShapeAxes,
	UIShapeEllipse,
	UIShapeFunction,
	UIShapeLabel,
	UIShapePath,
	UIShapePoint,
	UIShapePolygon,
	UIShapeSegment,
}

// uiFigureViews is the whitelist of 3D view presets.
var uiFigureViews = map[string]bool{"front": true, "top": true, "side": true, "iso": true}

// uiFigureColors is the whitelist of semantic stroke/fill colour names. The
// frontend maps each name onto a theme-aware CSS class.
var uiFigureColors = map[string]bool{
	"accent": true, "info": true, "warn": true, "success": true,
	"error": true, "dim": true, "none": true,
}

// uiFigureDashes is the whitelist of dash styles.
var uiFigureDashes = map[string]bool{"solid": true, "dashed": true, "dotted": true}

// uiFigureArrows is the whitelist of arrow placements.
var uiFigureArrows = map[string]bool{"end": true, "start": true, "both": true}

// uiFigureAnchors is the whitelist of text anchor values.
var uiFigureAnchors = map[string]bool{"start": true, "middle": true, "end": true}

// isUIShape reports whether typ is a figure shape.
func isUIShape(typ string) bool {
	for _, t := range uiShapeTypes {
		if t == typ {
			return true
		}
	}
	return false
}

// UIShapeCatalog returns the shape whitelist as a comma-separated list, for
// error messages and the system prompt.
func UIShapeCatalog() string {
	return strings.Join(uiShapeTypes, ", ")
}

// decodeProps decodes raw node props into a generic map using json.Number, so
// numeric fields are checked without a lossy float round trip.
func decodeProps(n *UINode) (map[string]interface{}, error) {
	props := map[string]interface{}{}
	if len(n.Props) == 0 {
		return props, nil
	}
	dec := json.NewDecoder(bytes.NewReader(n.Props))
	dec.UseNumber()
	if err := dec.Decode(&props); err != nil {
		return nil, fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrPropsObject, n.Type))
	}
	return props, nil
}

// numProp reads an optional (required=false) or mandatory finite number.
func numProp(nodeType string, props map[string]interface{}, key string, required bool) (float64, bool, error) {
	v, ok := props[key]
	if !ok || v == nil {
		if required {
			return 0, false, fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrRequiredProp, nodeType, key))
		}
		return 0, false, nil
	}
	num, isNum := v.(json.Number)
	if !isNum {
		return 0, true, fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrNumberProp, nodeType, key, propText(v)))
	}
	f, err := num.Float64()
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, true, fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrNumberProp, nodeType, key, num.String()))
	}
	return f, true, nil
}

// requireNumbers fails when any named field is missing or not a finite number.
func requireNumbers(nodeType string, props map[string]interface{}, keys ...string) error {
	for _, key := range keys {
		if _, _, err := numProp(nodeType, props, key, true); err != nil {
			return err
		}
	}
	return nil
}

// optionalNumbers validates the named fields when they are present.
func optionalNumbers(nodeType string, props map[string]interface{}, keys ...string) error {
	for _, key := range keys {
		if _, _, err := numProp(nodeType, props, key, false); err != nil {
			return err
		}
	}
	return nil
}

// rangeProp rejects a value outside [min, max].
func rangeProp(nodeType, key string, v, min, max float64) error {
	if v < min || v > max {
		return rangePropText(nodeType, key, numText(min)+" ~ "+numText(max), v)
	}
	return nil
}

// rangePropText rejects a value with a caller-supplied bounds description.
func rangePropText(nodeType, key, bounds string, v float64) error {
	return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrRangeProp, nodeType, key, bounds, numText(v)))
}

// boolProp validates that an optional prop is a JSON boolean.
func boolProp(nodeType string, props map[string]interface{}, key string) error {
	v, ok := props[key]
	if !ok || v == nil {
		return nil
	}
	if _, isBool := v.(bool); !isBool {
		return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrPropType, nodeType, key, "bool"))
	}
	return nil
}

// enumProp validates that an optional string prop is one of the allowed values.
func enumProp(nodeType string, props map[string]interface{}, key, allowed string, values map[string]bool) error {
	v, ok := props[key]
	if !ok || v == nil {
		return nil
	}
	s, isStr := v.(string)
	if !isStr || !values[s] {
		return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrPropType, nodeType, key, allowed))
	}
	return nil
}

// requireText validates a required string prop.
func requireText(nodeType string, props map[string]interface{}, key string) error {
	v, ok := props[key]
	if !ok || v == nil {
		return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrRequiredProp, nodeType, key))
	}
	if _, isStr := v.(string); !isStr {
		return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrPropType, nodeType, key, "string"))
	}
	return nil
}

// propText renders a raw prop value for an error message, truncated so a huge
// payload cannot inflate the error text.
func propText(v interface{}) string {
	s, ok := v.(string)
	if !ok {
		s = fmt.Sprintf("%v", v)
	}
	if len(s) > 60 {
		return s[:60] + "…"
	}
	return s
}

// numText renders a float in the shortest round-trippable form.
func numText(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// validateFigureProps checks the canvas props of a figure node.
func validateFigureProps(n *UINode) error {
	props, err := decodeProps(n)
	if err != nil {
		return err
	}
	if err := optionalNumbers(n.Type, props, "xmin", "xmax", "ymin", "ymax", "zmin", "zmax", "yaw", "pitch"); err != nil {
		return err
	}
	for _, pair := range [][2]string{{"xmin", "xmax"}, {"ymin", "ymax"}, {"zmin", "zmax"}} {
		lo, hasLo, err := numProp(n.Type, props, pair[0], false)
		if err != nil {
			return err
		}
		hi, hasHi, err := numProp(n.Type, props, pair[1], false)
		if err != nil {
			return err
		}
		if hasLo && hasHi && lo >= hi {
			return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrFigureRange, pair[0], pair[1]))
		}
	}
	for _, key := range []string{"width", "height"} {
		v, ok, err := numProp(n.Type, props, key, false)
		if err != nil {
			return err
		}
		if ok {
			if err := rangeProp(n.Type, key, v, 1, 10000); err != nil {
				return err
			}
		}
	}
	if err := enumProp(n.Type, props, "view", "front|top|side|iso", uiFigureViews); err != nil {
		return err
	}
	for _, key := range []string{"axes", "grid", "download"} {
		if err := boolProp(n.Type, props, key); err != nil {
			return err
		}
	}
	return nil
}

// validateShapeProps checks the props of one figure shape, by shape type.
func validateShapeProps(n *UINode) error {
	props, err := decodeProps(n)
	if err != nil {
		return err
	}
	switch n.Type {
	case UIShapePoint:
		if err := requireNumbers(n.Type, props, "x", "y"); err != nil {
			return err
		}
		if err := optionalNumbers(n.Type, props, "z"); err != nil {
			return err
		}
		if r, ok, err := numProp(n.Type, props, "r", false); err != nil {
			return err
		} else if ok && (r <= 0 || r > 100) {
			return rangePropText(n.Type, "r", "(0, 100]", r)
		}
		if err := boolProp(n.Type, props, "filled"); err != nil {
			return err
		}
	case UIShapeSegment:
		if err := requireNumbers(n.Type, props, "x1", "y1", "x2", "y2"); err != nil {
			return err
		}
		if err := optionalNumbers(n.Type, props, "z1", "z2"); err != nil {
			return err
		}
		if err := checkArrowProp(n.Type, props); err != nil {
			return err
		}
	case UIShapePolygon, UIShapePath:
		if err := checkPointsProp(n.Type, props); err != nil {
			return err
		}
		for _, key := range []string{"closed", "smooth"} {
			if err := boolProp(n.Type, props, key); err != nil {
				return err
			}
		}
		if err := checkArrowProp(n.Type, props); err != nil {
			return err
		}
	case UIShapeEllipse:
		if err := requireNumbers(n.Type, props, "cx", "cy", "rx", "ry"); err != nil {
			return err
		}
		for _, key := range []string{"rx", "ry"} {
			v, _, err := numProp(n.Type, props, key, true)
			if err != nil {
				return err
			}
			if v < 0 {
				return rangePropText(n.Type, key, "≥ 0", v)
			}
		}
		if err := optionalNumbers(n.Type, props, "z", "rotation"); err != nil {
			return err
		}
	case UIShapeAxes:
		for _, key := range []string{"x", "y", "z", "grid"} {
			if err := boolProp(n.Type, props, key); err != nil {
				return err
			}
		}
		if v, ok, err := numProp(n.Type, props, "ticks", false); err != nil {
			return err
		} else if ok {
			if err := rangeProp(n.Type, "ticks", v, 0, 200); err != nil {
				return err
			}
		}
		if v, ok, err := numProp(n.Type, props, "step", false); err != nil {
			return err
		} else if ok && v <= 0 {
			return rangePropText(n.Type, "step", "> 0", v)
		}
		if err := checkOriginProp(n.Type, props); err != nil {
			return err
		}
	case UIShapeLabel:
		if err := requireNumbers(n.Type, props, "x", "y"); err != nil {
			return err
		}
		if err := optionalNumbers(n.Type, props, "z", "dx", "dy"); err != nil {
			return err
		}
		if err := requireText(n.Type, props, "text"); err != nil {
			return err
		}
		if err := enumProp(n.Type, props, "anchor", "start|middle|end", uiFigureAnchors); err != nil {
			return err
		}
		if v, ok, err := numProp(n.Type, props, "size", false); err != nil {
			return err
		} else if ok {
			if err := rangeProp(n.Type, "size", v, 1, 200); err != nil {
				return err
			}
		}
	case UIShapeFunction:
		if err := checkExprProp(n.Type, props); err != nil {
			return err
		}
		if err := requireNumbers(n.Type, props, "from", "to"); err != nil {
			return err
		}
		if err := optionalNumbers(n.Type, props, "z"); err != nil {
			return err
		}
		if v, ok, err := numProp(n.Type, props, "samples", false); err != nil {
			return err
		} else if ok {
			if err := rangeProp(n.Type, "samples", v, 1, UIMaxShapePoints); err != nil {
				return err
			}
		}
	}
	return validateShapeStyle(n.Type, props)
}

// checkPointsProp validates the required "points" array of polygon/path.
func checkPointsProp(nodeType string, props map[string]interface{}) error {
	v, ok := props["points"]
	if !ok || v == nil {
		return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrRequiredProp, nodeType, "points"))
	}
	pts, isSlice := v.([]interface{})
	if !isSlice || len(pts) == 0 {
		return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrPointsFormat, nodeType))
	}
	if len(pts) > UIMaxShapePoints {
		return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrPointsLimit, nodeType, UIMaxShapePoints))
	}
	for _, item := range pts {
		pt, isPoint := item.([]interface{})
		if !isPoint || len(pt) < 2 || len(pt) > 3 {
			return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrPointsFormat, nodeType))
		}
		for _, coord := range pt {
			if !isFiniteCoord(coord) {
				return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrPointsFormat, nodeType))
			}
		}
	}
	return nil
}

// checkOriginProp validates the optional "origin" point of an axes shape.
func checkOriginProp(nodeType string, props map[string]interface{}) error {
	v, ok := props["origin"]
	if !ok || v == nil {
		return nil
	}
	pt, isSlice := v.([]interface{})
	if !isSlice || len(pt) < 2 || len(pt) > 3 {
		return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrPointsFormat, nodeType))
	}
	for _, coord := range pt {
		if !isFiniteCoord(coord) {
			return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrPointsFormat, nodeType))
		}
	}
	return nil
}

// checkExprProp validates the required "expr" of a function shape: one
// expression string, or a 2-3 element parametric array.
func checkExprProp(nodeType string, props map[string]interface{}) error {
	v, ok := props["expr"]
	if !ok || v == nil {
		return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrRequiredProp, nodeType, "expr"))
	}
	switch expr := v.(type) {
	case string:
		if len(expr) > UIMaxExprLength {
			return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrExprTooLong, UIMaxExprLength))
		}
	case []interface{}:
		if len(expr) < 2 || len(expr) > 3 {
			return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrPropType, nodeType, "expr", "string | [string, string, string]"))
		}
		for _, item := range expr {
			s, isStr := item.(string)
			if !isStr || len(s) > UIMaxExprLength {
				return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrExprTooLong, UIMaxExprLength))
			}
		}
	default:
		return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrPropType, nodeType, "expr", "string | [string, string, string]"))
	}
	if exprZ, ok := props["exprZ"]; ok && exprZ != nil {
		s, isStr := exprZ.(string)
		if !isStr || len(s) > UIMaxExprLength {
			return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrExprTooLong, UIMaxExprLength))
		}
	}
	return nil
}

// checkArrowProp validates the optional "arrow" prop: a boolean or a placement
// name (end/start/both).
func checkArrowProp(nodeType string, props map[string]interface{}) error {
	v, ok := props["arrow"]
	if !ok || v == nil {
		return nil
	}
	if _, isBool := v.(bool); isBool {
		return nil
	}
	s, isStr := v.(string)
	if !isStr || !uiFigureArrows[s] {
		return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrPropType, nodeType, "arrow", "false|end|start|both"))
	}
	return nil
}

// validateShapeStyle checks the style props shared by every shape.
func validateShapeStyle(nodeType string, props map[string]interface{}) error {
	for _, key := range []string{"stroke", "fill"} {
		if err := enumProp(nodeType, props, key, "accent|info|warn|success|error|dim|none", uiFigureColors); err != nil {
			return err
		}
	}
	if err := enumProp(nodeType, props, "dash", "solid|dashed|dotted", uiFigureDashes); err != nil {
		return err
	}
	if w, ok, err := numProp(nodeType, props, "strokeWidth", false); err != nil {
		return err
	} else if ok && (w <= 0 || w > 6) {
		return rangePropText(nodeType, "strokeWidth", "(0, 6]", w)
	}
	if o, ok, err := numProp(nodeType, props, "opacity", false); err != nil {
		return err
	} else if ok && (o < 0 || o > 1) {
		return rangePropText(nodeType, "opacity", "0 ~ 1", o)
	}
	return nil
}

// isFiniteCoord reports whether a raw JSON value is a finite number.
func isFiniteCoord(raw interface{}) bool {
	num, ok := raw.(json.Number)
	if !ok {
		return false
	}
	f, err := num.Float64()
	return err == nil && !math.IsNaN(f) && !math.IsInf(f, 0)
}
