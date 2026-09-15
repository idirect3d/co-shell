// co-shell Web UI - form component for LLM component trees (FEATURE-524).
//
// Registers the "form" component into the ui.js registry. The LLM declares the
// fields; this file renders native controls (createElement only, never
// innerHTML), collects their values on submit and hands them to
// UI.sendUIAction, which sends {"type":"ui_action", ui_id, action_id, payload}
// upstream. The backend turns that into a new agent turn, so the LLM sees the
// structured values and continues the task (decision 5: continuation semantics).
//
// Field types: text | number | textarea | select | checkbox.
//
// Author: L.Shuang
// Created: 2026-09-15
// MIT License - Copyright (c) 2026 L.Shuang

(function (global) {
  "use strict";

  if (!global.UI || typeof global.UI.register !== "function") return;

  function t(zh, en) {
    return typeof global.UI.t === "function" ? global.UI.t(zh, en) : zh;
  }

  function el(tag, cls, text) {
    var node = document.createElement(tag);
    if (cls) node.className = cls;
    if (text !== undefined && text !== null) node.textContent = String(text);
    return node;
  }

  function str(v) {
    return v === undefined || v === null ? "" : String(v);
  }

  // optionValue accepts "方案A", {value, label} (the documented shape) or the
  // legacy {v, label} alias. Reading only one of the two keys silently turned
  // every option value into "", so the user's choice never reached the LLM.
  function optionValue(o) {
    if (!o || typeof o !== "object") return str(o);
    if (o.value !== undefined && o.value !== null) return str(o.value);
    return str(o.v);
  }

  function optionLabel(o) {
    if (!o || typeof o !== "object") return str(o);
    if (o.label !== undefined && o.label !== null) return str(o.label);
    if (o.value !== undefined && o.value !== null) return str(o.value);
    return str(o.v);
  }

  // buildControl renders one field and returns the element whose value is read
  // back on submit.
  function buildControl(field) {
    var type = str(field.type || "text").toLowerCase();
    var control;
    if (type === "textarea") {
      control = el("textarea", "ui-input ui-textarea");
      control.rows = field.rows ? parseInt(field.rows, 10) || 3 : 3;
    } else if (type === "select") {
      control = el("select", "ui-input");
      var options = Array.isArray(field.options) ? field.options : [];
      for (var i = 0; i < options.length; i++) {
        var opt = el("option", "", optionLabel(options[i]));
        opt.value = optionValue(options[i]);
        control.appendChild(opt);
      }
    } else if (type === "checkbox") {
      control = el("input", "ui-check-input");
      control.type = "checkbox";
    } else if (type === "number") {
      control = el("input", "ui-input");
      control.type = "number";
    } else {
      control = el("input", "ui-input");
      control.type = "text";
    }
    control.setAttribute("data-field", str(field.name));
    if (field.placeholder !== undefined && field.placeholder !== null && type !== "checkbox") {
      control.placeholder = str(field.placeholder);
    }
    if (type === "checkbox") {
      control.checked = field.value === true || field.value === "true";
    } else if (field.value !== undefined && field.value !== null) {
      control.value = str(field.value);
    }
    return control;
  }

  // readValue normalizes a control's value for the payload.
  function readValue(control) {
    if (control.type === "checkbox") return !!control.checked;
    if (control.type === "number") {
      var n = parseFloat(control.value);
      return isFinite(n) ? n : null;
    }
    return control.value;
  }

  global.UI.register("form", {
    children: false,
    render: function (node) {
      var props = node.props;
      var fields = Array.isArray(props.fields) ? props.fields : [];
      // ui.js calls render() with {props, id, raw}: the declared actions live on
      // the raw node (reading node.actions here silently disabled the submit
      // button — the form looked fine but could never send anything).
      var raw = node.raw && typeof node.raw === "object" ? node.raw : {};
      var actions = Array.isArray(raw.actions) ? raw.actions : [];
      var submitAction = null;
      for (var i = 0; i < actions.length; i++) {
        if (actions[i] && actions[i].on === "submit") { submitAction = actions[i]; break; }
      }

      var form = el("form", "ui-form");
      form.noValidate = true; // the LLM asks for data, not browser-side validation
      var title = str(props.title);
      if (title) form.appendChild(el("div", "ui-form-title", title));

      var controls = []; // parallel to fields
      for (var f = 0; f < fields.length; f++) {
        var field = fields[f] && typeof fields[f] === "object" ? fields[f] : {};
        var type = str(field.type || "text").toLowerCase();
        var control = buildControl(field);
        controls.push(control);
        var wrap = el("label", "ui-field ui-field-" + type);
        var label = el("span", "ui-field-label", str(field.label !== undefined ? field.label : field.name));
        if (type === "checkbox") {
          // checkbox: control first, then its label text (native reading order)
          wrap.appendChild(control);
          wrap.appendChild(label);
        } else {
          wrap.appendChild(label);
          wrap.appendChild(control);
        }
        form.appendChild(wrap);
      }

      var actionsBar = el("div", "ui-form-actions");
      var submit = el("button", "btn primary ui-form-submit", str(props.submit) || t("提交", "Submit"));
      submit.type = "submit";
      if (!submitAction) {
        // No declared action: keep the button inert instead of sending nothing.
        submit.disabled = true;
        submit.title = t("该表单未声明提交动作", "This form declares no submit action");
      }
      actionsBar.appendChild(submit);
      if (str(props.reset)) {
        var reset = el("button", "btn ghost ui-form-reset", str(props.reset));
        reset.type = "reset";
        actionsBar.appendChild(reset);
      }
      form.appendChild(actionsBar);

      form.onsubmit = function (e) {
        e.preventDefault();
        if (!submitAction || !submitAction.id) return;
        var payload = {};
        for (var k = 0; k < fields.length; k++) {
          var name = str(fields[k] && fields[k].name);
          if (!name) continue;
          payload[name] = readValue(controls[k]);
        }
        var sent = typeof global.UI.sendUIAction === "function"
          ? global.UI.sendUIAction(submitAction.id, payload, form)
          : false;
        if (sent) {
          submit.disabled = true;
          form.setAttribute("data-submitted", "1");
        }
      };
      return form;
    }
  });
})(window);
