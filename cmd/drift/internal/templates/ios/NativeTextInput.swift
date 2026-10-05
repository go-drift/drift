/// NativeTextInput.swift
/// Provides native text input views embedded in Drift UI with Skia chrome.

import DriftPluginAPI
import UIKit

// MARK: - Padded Text Field

/// Manages stable pointer IDs for forwarded touches.
/// UITouch.hash is not guaranteed stable, so we map touches to monotonically increasing IDs.
class TouchPointerIDManager {
    static let shared = TouchPointerIDManager()
    private var touchToID: [ObjectIdentifier: Int64] = [:]
    private var nextID: Int64 = 1_000_000 // Start high to avoid collision with native pointer IDs

    func getID(for touch: UITouch) -> Int64 {
        let key = ObjectIdentifier(touch)
        if let id = touchToID[key] {
            return id
        }
        let id = nextID
        nextID += 1
        touchToID[key] = id
        return id
    }

    func releaseID(for touch: UITouch) {
        touchToID.removeValue(forKey: ObjectIdentifier(touch))
    }
}

/// UITextField subclass with configurable padding.
class PaddedTextField: UITextField {
    var padding: UIEdgeInsets = .zero

    override func textRect(forBounds bounds: CGRect) -> CGRect {
        return bounds.inset(by: padding)
    }

    override func editingRect(forBounds bounds: CGRect) -> CGRect {
        return bounds.inset(by: padding)
    }

    override func placeholderRect(forBounds bounds: CGRect) -> CGRect {
        return bounds.inset(by: padding)
    }
}

// MARK: - Padded Text View

/// UITextView subclass with configurable padding via textContainerInset.
class PaddedTextView: UITextView {
    var placeholderLabel: UILabel?
    var placeholderColor: UIColor = UIColor(white: 0.6, alpha: 1.0)
    var placeholderText: String = "" {
        didSet {
            placeholderLabel?.text = placeholderText
            updatePlaceholder()
        }
    }

    override var text: String! {
        didSet { updatePlaceholder() }
    }

    func updatePlaceholder() {
        placeholderLabel?.isHidden = !text.isEmpty
    }

    func setupPlaceholder() {
        let label = UILabel()
        label.text = placeholderText
        label.textColor = placeholderColor
        label.font = font
        label.numberOfLines = 0
        label.translatesAutoresizingMaskIntoConstraints = false
        addSubview(label)

        NSLayoutConstraint.activate([
            label.topAnchor.constraint(equalTo: topAnchor, constant: textContainerInset.top),
            label.leadingAnchor.constraint(equalTo: leadingAnchor, constant: textContainerInset.left + textContainer.lineFragmentPadding),
            label.trailingAnchor.constraint(lessThanOrEqualTo: trailingAnchor, constant: -textContainerInset.right)
        ])

        placeholderLabel = label
        updatePlaceholder()
    }
}

// MARK: - Native Text Input Container

/// Platform view container for native text input.
class NativeTextInputContainer: NSObject, DriftPlatformView, UITextFieldDelegate, UITextViewDelegate {
    let viewId: Int
    let view: UIView

    private var textField: PaddedTextField?
    private var textView: PaddedTextView?
    private var isMultiline: Bool = false
    private var suppressCallback: Bool = false
    private var config: TextInputViewConfig

    init(viewId: Int, params: [String: Any]) {
        self.viewId = viewId
        self.config = TextInputViewConfig(params: params)
        self.isMultiline = config.multiline

        if config.multiline {
            let tv = PaddedTextView()
            tv.backgroundColor = .clear
            tv.font = config.font
            tv.textColor = config.textColor
            tv.textAlignment = config.textAlignment
            tv.keyboardType = config.keyboardType
            tv.returnKeyType = config.returnKeyType
            tv.autocorrectionType = config.autocorrect ? .yes : .no
            tv.autocapitalizationType = config.capitalization
            tv.isSecureTextEntry = config.obscure
            tv.textContainerInset = config.padding
            tv.placeholderText = config.placeholder
            tv.placeholderColor = config.placeholderColor
            self.view = tv
            self.textView = tv

            super.init()
            tv.delegate = self
            tv.setupPlaceholder()
        } else {
            let tf = PaddedTextField()
            tf.backgroundColor = .clear
            tf.borderStyle = .none
            tf.font = config.font
            tf.textColor = config.textColor
            tf.textAlignment = config.textAlignment
            tf.keyboardType = config.keyboardType
            tf.returnKeyType = config.returnKeyType
            tf.autocorrectionType = config.autocorrect ? .yes : .no
            tf.autocapitalizationType = config.capitalization
            tf.isSecureTextEntry = config.obscure
            tf.padding = config.padding
            tf.placeholder = config.placeholder
            tf.attributedPlaceholder = NSAttributedString(
                string: config.placeholder,
                attributes: [.foregroundColor: config.placeholderColor]
            )
            self.view = tf
            self.textField = tf

            super.init()
            tf.delegate = self
            tf.addTarget(self, action: #selector(textDidChange), for: .editingChanged)
        }

        // Apply initial text if provided
        if let text = params["text"] as? String {
            setText(text)
        }
    }

    func dispose() {
        textField?.resignFirstResponder()
        textView?.resignFirstResponder()
        view.removeFromSuperview()
    }

    func onGeometryChanged() {
        guard isMultiline, let tv = textView else { return }

        // Refresh text layout immediately after host geometry updates so
        // UIScrollView metrics stay consistent with resized bounds.
        tv.layoutIfNeeded()

        // If UIKit keeps stale scrollability after a bounds change, toggling
        // isScrollEnabled forces UIScrollView to rebuild scroll state.
        // Preserve the original setting.
        let wasScrollEnabled = tv.isScrollEnabled
        tv.isScrollEnabled = false
        tv.isScrollEnabled = wasScrollEnabled

        let insets = tv.adjustedContentInset
        let minY = -insets.top
        let maxY = max(minY, tv.contentSize.height - tv.bounds.height + insets.bottom)
        let clampedY = min(max(tv.contentOffset.y, minY), maxY)

        if clampedY != tv.contentOffset.y {
            tv.setContentOffset(CGPoint(x: tv.contentOffset.x, y: clampedY), animated: false)
        }
    }

    // MARK: - View Methods

    func setText(_ text: String) {
        suppressCallback = true
        if isMultiline {
            textView?.text = text
        } else {
            textField?.text = text
        }
        suppressCallback = false
    }

    func setSelection(base: Int, extent: Int) {
        if isMultiline {
            guard let tv = textView else { return }
            if let start = tv.position(from: tv.beginningOfDocument, offset: base),
               let end = tv.position(from: tv.beginningOfDocument, offset: extent) {
                tv.selectedTextRange = tv.textRange(from: start, to: end)
            }
        } else {
            guard let tf = textField else { return }
            if let start = tf.position(from: tf.beginningOfDocument, offset: base),
               let end = tf.position(from: tf.beginningOfDocument, offset: extent) {
                tf.selectedTextRange = tf.textRange(from: start, to: end)
            }
        }
    }

    func setValue(text: String, selectionBase: Int, selectionExtent: Int) {
        setText(text)
        setSelection(base: selectionBase, extent: selectionExtent)
    }

    func focus() {
        if isMultiline {
            textView?.becomeFirstResponder()
        } else {
            textField?.becomeFirstResponder()
        }
    }

    func blur() {
        if isMultiline {
            textView?.resignFirstResponder()
        } else {
            textField?.resignFirstResponder()
        }
    }

    func updateConfig(_ params: [String: Any]) {
        config = TextInputViewConfig(params: params)

        if isMultiline {
            guard let tv = textView else { return }
            tv.font = config.font
            tv.textColor = config.textColor
            tv.textAlignment = config.textAlignment
            tv.keyboardType = config.keyboardType
            tv.returnKeyType = config.returnKeyType
            tv.autocorrectionType = config.autocorrect ? .yes : .no
            tv.autocapitalizationType = config.capitalization
            tv.isSecureTextEntry = config.obscure
            tv.textContainerInset = config.padding
            tv.placeholderText = config.placeholder
            tv.placeholderColor = config.placeholderColor
            tv.placeholderLabel?.textColor = config.placeholderColor
        } else {
            guard let tf = textField else { return }
            tf.font = config.font
            tf.textColor = config.textColor
            tf.textAlignment = config.textAlignment
            tf.keyboardType = config.keyboardType
            tf.returnKeyType = config.returnKeyType
            tf.autocorrectionType = config.autocorrect ? .yes : .no
            tf.autocapitalizationType = config.capitalization
            tf.isSecureTextEntry = config.obscure
            tf.padding = config.padding
            tf.attributedPlaceholder = NSAttributedString(
                string: config.placeholder,
                attributes: [.foregroundColor: config.placeholderColor]
            )
        }
    }

    // MARK: - Event Handling

    @objc private func textDidChange() {
        guard !suppressCallback, let tf = textField else { return }
        sendTextChanged(text: tf.text ?? "", textInput: tf)
    }

    private func sendTextChanged(text: String, textInput: UITextInput) {
        var selBase = text.count
        var selExtent = text.count

        if let range = textInput.selectedTextRange {
            selBase = textInput.offset(from: textInput.beginningOfDocument, to: range.start)
            selExtent = textInput.offset(from: textInput.beginningOfDocument, to: range.end)
        }

        PlatformChannelManager.shared.sendEvent(
            channel: "drift/platform_views",
            data: [
                "method": "onTextChanged",
                "viewId": viewId,
                "text": text,
                "selectionBase": selBase,
                "selectionExtent": selExtent
            ]
        )
    }

    private func sendAction(_ action: Int) {
        PlatformChannelManager.shared.sendEvent(
            channel: "drift/platform_views",
            data: [
                "method": "onAction",
                "viewId": viewId,
                "action": action
            ]
        )
    }

    private func sendFocusChanged(_ focused: Bool) {
        PlatformChannelManager.shared.sendEvent(
            channel: "drift/platform_views",
            data: [
                "method": "onFocusChanged",
                "viewId": viewId,
                "focused": focused
            ]
        )
    }

    // MARK: - UITextFieldDelegate

    func textFieldDidBeginEditing(_ textField: UITextField) {
        sendFocusChanged(true)
    }

    func textFieldDidEndEditing(_ textField: UITextField) {
        sendFocusChanged(false)
    }

    func textFieldShouldReturn(_ textField: UITextField) -> Bool {
        let action = actionFromReturnKeyType(config.returnKeyType)
        sendAction(action)

        // Dismiss for done/go/search/send actions
        switch config.returnKeyType {
        case .done, .go, .search, .send:
            textField.resignFirstResponder()
        default:
            break
        }

        return false
    }

    // MARK: - UITextViewDelegate

    func textViewDidBeginEditing(_ textView: UITextView) {
        sendFocusChanged(true)
    }

    func textViewDidEndEditing(_ textView: UITextView) {
        sendFocusChanged(false)
    }

    func textViewDidChange(_ textView: UITextView) {
        guard !suppressCallback else { return }
        if let ptv = textView as? PaddedTextView {
            ptv.updatePlaceholder()
        }
        sendTextChanged(text: textView.text, textInput: textView)
    }

    // MARK: - Helpers

    private func actionFromReturnKeyType(_ type: UIReturnKeyType) -> Int {
        switch type {
        case .done: return 1
        case .go: return 2
        case .next: return 3
        case .search: return 5
        case .send: return 6
        default: return 7 // newline
        }
    }
}

// MARK: - Text Input View Config

/// Configuration for native text input view.
struct TextInputViewConfig {
    let fontFamily: String
    let fontSize: CGFloat
    let fontWeight: UIFont.Weight
    let textColor: UIColor
    let placeholderColor: UIColor
    let textAlignment: NSTextAlignment
    let multiline: Bool
    let maxLines: Int
    let obscure: Bool
    let autocorrect: Bool
    let keyboardType: UIKeyboardType
    let returnKeyType: UIReturnKeyType
    let capitalization: UITextAutocapitalizationType
    let padding: UIEdgeInsets
    let placeholder: String

    var font: UIFont {
        if fontFamily.isEmpty {
            return UIFont.systemFont(ofSize: fontSize, weight: fontWeight)
        }
        if let font = UIFont(name: fontFamily, size: fontSize) {
            return font
        }
        return UIFont.systemFont(ofSize: fontSize, weight: fontWeight)
    }

    init(params: [String: Any]) {
        fontFamily = params["fontFamily"] as? String ?? ""
        fontSize = CGFloat(params["fontSize"] as? Double ?? 16.0)

        let weightValue = params["fontWeight"] as? Int ?? 400
        switch weightValue {
        case 100: fontWeight = .ultraLight
        case 200: fontWeight = .thin
        case 300: fontWeight = .light
        case 400: fontWeight = .regular
        case 500: fontWeight = .medium
        case 600: fontWeight = .semibold
        case 700: fontWeight = .bold
        case 800: fontWeight = .heavy
        case 900: fontWeight = .black
        default: fontWeight = .regular
        }

        if let color = params["textColor"] as? UInt32 {
            textColor = UIColor(argb: color)
        } else {
            textColor = .black
        }

        if let color = params["placeholderColor"] as? UInt32 {
            placeholderColor = UIColor(argb: color)
        } else {
            placeholderColor = UIColor(white: 0.6, alpha: 1.0)
        }

        let alignValue = params["textAlignment"] as? Int ?? 0
        switch alignValue {
        case 1: textAlignment = .center
        case 2: textAlignment = .right
        default: textAlignment = .left
        }

        multiline = params["multiline"] as? Bool ?? false
        maxLines = params["maxLines"] as? Int ?? 0
        obscure = params["obscure"] as? Bool ?? false
        autocorrect = params["autocorrect"] as? Bool ?? true

        let kbType = params["keyboardType"] as? Int ?? 0
        switch kbType {
        case 1: keyboardType = .numberPad
        case 2: keyboardType = .phonePad
        case 3: keyboardType = .emailAddress
        case 4: keyboardType = .URL
        case 5: keyboardType = .default // password
        case 6: keyboardType = .default // multiline
        default: keyboardType = .default
        }

        let actionType = params["inputAction"] as? Int ?? 1
        switch actionType {
        case 1: returnKeyType = .done
        case 2: returnKeyType = .go
        case 3: returnKeyType = .next
        case 4: returnKeyType = .default // previous
        case 5: returnKeyType = .search
        case 6: returnKeyType = .send
        case 7: returnKeyType = .default // newline
        default: returnKeyType = .done
        }

        let capType = params["capitalization"] as? Int ?? 3
        switch capType {
        case 0: capitalization = .none
        case 1: capitalization = .allCharacters
        case 2: capitalization = .words
        case 3: capitalization = .sentences
        default: capitalization = .sentences
        }

        let paddingLeft = CGFloat(params["paddingLeft"] as? Double ?? 0)
        let paddingTop = CGFloat(params["paddingTop"] as? Double ?? 0)
        let paddingRight = CGFloat(params["paddingRight"] as? Double ?? 0)
        let paddingBottom = CGFloat(params["paddingBottom"] as? Double ?? 0)
        padding = UIEdgeInsets(top: paddingTop, left: paddingLeft, bottom: paddingBottom, right: paddingRight)

        placeholder = params["placeholder"] as? String ?? ""
    }
}

// MARK: - UIColor Extension

extension UIColor {
    convenience init(argb: UInt32) {
        let alpha = CGFloat((argb >> 24) & 0xFF) / 255.0
        let red = CGFloat((argb >> 16) & 0xFF) / 255.0
        let green = CGFloat((argb >> 8) & 0xFF) / 255.0
        let blue = CGFloat(argb & 0xFF) / 255.0
        self.init(red: red, green: green, blue: blue, alpha: alpha)
    }
}

extension NativeTextInputContainer {
    func handle(_ method: String, _ args: [String: Any]) throws {
        switch method {
        case "setText":
            if let text = args["text"] as? String {
                setText(text)
            }
        case "setSelection":
            setSelection(base: args["selectionBase"] as? Int ?? 0, extent: args["selectionExtent"] as? Int ?? 0)
        case "setValue":
            let text = args["text"] as? String ?? ""
            setValue(text: text,
                     selectionBase: args["selectionBase"] as? Int ?? text.count,
                     selectionExtent: args["selectionExtent"] as? Int ?? text.count)
        case "focus": focus()
        case "blur": blur()
        case "updateConfig": updateConfig(args)
        default: throw unknownViewMethod(method)
        }
    }
}
