/// TextInput.swift
/// Provides native text input (IME) handling for the Drift framework.

import UIKit

// MARK: - Text Input Handler

/// Handles text input channel methods from Go.
enum TextInputHandler {
    private static var connections: [Int: TextInputConnection] = [:]

    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any] else {
            return (nil, NSError(domain: "TextInput", code: 400, userInfo: [NSLocalizedDescriptionKey: "Invalid arguments"]))
        }

        switch method {
        case "createConnection":
            return createConnection(args: dict)
        case "closeConnection":
            return closeConnection(args: dict)
        case "show":
            return show(args: dict)
        case "hide":
            return hide(args: dict)
        case "setEditingState":
            return setEditingState(args: dict)
        default:
            return (nil, NSError(domain: "TextInput", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }
    }

    /// Safely extracts an integer from various numeric types in JSON decoded data.
    private static func getInt(_ value: Any?) -> Int? {
        switch value {
        case let i as Int: return i
        case let i as Int64: return Int(i)
        case let i as Int32: return Int(i)
        case let i as UInt: return Int(i)
        case let i as UInt64: return Int(i)
        case let i as Double: return Int(i)
        case let i as Float: return Int(i)
        case let n as NSNumber: return n.intValue
        default: return nil
        }
    }

    private static func keyboardTypeFrom(value: Int) -> UIKeyboardType {
        switch value {
        case 1: return .numberPad
        case 2: return .phonePad
        case 3: return .emailAddress
        case 4: return .URL
        default: return .default
        }
    }

    private static func returnKeyTypeFrom(value: Int) -> UIReturnKeyType {
        switch value {
        case 1: return .done
        case 2: return .go
        case 3: return .next
        case 4: return .default
        case 5: return .search
        case 6: return .send
        default: return .default
        }
    }

    private static func autocapitalizationTypeFrom(value: Int) -> UITextAutocapitalizationType {
        switch value {
        case 0: return .none
        case 1: return .allCharacters
        case 2: return .words
        case 3: return .sentences
        default: return .sentences
        }
    }

    private static func createConnection(args: [String: Any]) -> (Any?, Error?) {
        guard let connectionId = getInt(args["connectionId"]) else {
            return (nil, NSError(domain: "TextInput", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing connectionId"]))
        }

        // Parse configuration
        let keyboardTypeValue = getInt(args["keyboardType"]) ?? 0
        let inputActionValue = getInt(args["inputAction"]) ?? 0
        let keyboardType = keyboardTypeFrom(value: keyboardTypeValue)
        let returnKeyType = returnKeyTypeFrom(value: inputActionValue)
        let autocorrect = args["autocorrect"] as? Bool ?? true
        let enableSuggestions = args["enableSuggestions"] as? Bool ?? true
        let obscure = args["obscure"] as? Bool ?? false
        let capitalization = autocapitalizationTypeFrom(value: getInt(args["capitalization"]) ?? 0)

        let config = TextInputConfiguration(
            keyboardType: keyboardType,
            returnKeyType: returnKeyType,
            autocorrectionType: autocorrect ? .yes : .no,
            spellCheckingType: enableSuggestions ? .yes : .no,
            isSecureTextEntry: obscure,
            autocapitalizationType: capitalization
        )

        let connection = TextInputConnection(connectionId: connectionId, config: config)
        connections[connectionId] = connection

        return (["created": true], nil)
    }

    private static func closeConnection(args: [String: Any]) -> (Any?, Error?) {
        guard let connectionId = getInt(args["connectionId"]) else {
            return (nil, nil)
        }

        if let connection = connections[connectionId] {
            connection.close()
            connections.removeValue(forKey: connectionId)
        }

        return (nil, nil)
    }

    private static func show(args: [String: Any]) -> (Any?, Error?) {
        guard let connectionId = getInt(args["connectionId"]),
              let connection = connections[connectionId] else {
            return (nil, nil)
        }

        connection.show()
        return (nil, nil)
    }

    private static func hide(args: [String: Any]) -> (Any?, Error?) {
        guard let connectionId = getInt(args["connectionId"]),
              let connection = connections[connectionId] else {
            return (nil, nil)
        }

        connection.hide()
        return (nil, nil)
    }

    private static func setEditingState(args: [String: Any]) -> (Any?, Error?) {
        guard let connectionId = getInt(args["connectionId"]),
              let connection = connections[connectionId] else {
            return (nil, nil)
        }

        let text = args["text"] as? String ?? ""
        let selectionBase = getInt(args["selectionBase"]) ?? text.count
        let selectionExtent = getInt(args["selectionExtent"]) ?? text.count
        let composingStart = getInt(args["composingStart"]) ?? -1
        let composingEnd = getInt(args["composingEnd"]) ?? -1

        let state = TextEditingState(
            text: text,
            selectionBase: selectionBase,
            selectionExtent: selectionExtent,
            composingStart: composingStart,
            composingEnd: composingEnd
        )

        connection.setEditingState(state)
        return (nil, nil)
    }
}

// MARK: - Text Input Configuration

struct TextInputConfiguration {
    let keyboardType: UIKeyboardType
    let returnKeyType: UIReturnKeyType
    let autocorrectionType: UITextAutocorrectionType
    let spellCheckingType: UITextSpellCheckingType
    let isSecureTextEntry: Bool
    let autocapitalizationType: UITextAutocapitalizationType
}

// MARK: - Text Editing State

struct TextEditingState {
    var text: String
    var selectionBase: Int
    var selectionExtent: Int
    var composingStart: Int
    var composingEnd: Int
}

// MARK: - Text Input Connection

/// Manages a single text input connection with the keyboard.
class TextInputConnection: NSObject {
    let connectionId: Int
    let config: TextInputConfiguration

    private var textField: HiddenTextField?
    private var editingState: TextEditingState
    private var suppressCallback: Bool = false  // Suppress callback during programmatic text changes

    init(connectionId: Int, config: TextInputConfiguration) {
        self.connectionId = connectionId
        self.config = config
        self.editingState = TextEditingState(text: "", selectionBase: 0, selectionExtent: 0, composingStart: -1, composingEnd: -1)
        super.init()
    }

    func show() {
        DispatchQueue.main.async { [weak self] in
            self?.showKeyboard()
        }
    }

    private func showKeyboard() {
        guard textField == nil else {
            textField?.becomeFirstResponder()
            return
        }

        guard let windowScene = UIApplication.shared.connectedScenes.first as? UIWindowScene,
              let window = windowScene.windows.first else {
            return
        }

        let field = HiddenTextField(frame: CGRect(x: -1000, y: -1000, width: 100, height: 50))
        field.keyboardType = config.keyboardType
        field.returnKeyType = config.returnKeyType
        field.autocorrectionType = config.autocorrectionType
        field.spellCheckingType = config.spellCheckingType
        field.isSecureTextEntry = config.isSecureTextEntry
        field.autocapitalizationType = config.autocapitalizationType
        if #available(iOS 10.0, *) {
            if config.isSecureTextEntry {
                field.textContentType = .password
            } else if config.keyboardType == .emailAddress {
                field.textContentType = .emailAddress
            }
        }
        field.delegate = self
        field.connectionId = connectionId
        field.onTextChange = { [weak self] text in
            self?.handleTextChange(text)
        }

        // Suppress callbacks during initial setup to prevent spurious empty updates
        suppressCallback = true
        field.text = editingState.text
        window.addSubview(field)
        textField = field
        field.becomeFirstResponder()
        suppressCallback = false
    }

    func hide() {
        DispatchQueue.main.async { [weak self] in
            self?.textField?.resignFirstResponder()
        }
    }

    func close() {
        DispatchQueue.main.async { [weak self] in
            guard let self = self, let field = self.textField else { return }

            // Ensure keyboard is dismissed before removing from view hierarchy
            if field.isFirstResponder {
                field.resignFirstResponder()
            }

            // Delay removal slightly to allow keyboard dismissal to complete
            DispatchQueue.main.async {
                field.removeFromSuperview()
                self.textField = nil
            }
        }
    }

    func setEditingState(_ state: TextEditingState) {
        editingState = state
        DispatchQueue.main.async { [weak self] in
            guard let self = self, let field = self.textField else { return }

            // Suppress callback during programmatic text change to prevent
            // sending the same value back to Go (which would trigger validation)
            self.suppressCallback = true
            field.text = state.text
            self.suppressCallback = false

            // Set selection
            if let start = field.position(from: field.beginningOfDocument, offset: state.selectionBase),
               let end = field.position(from: field.beginningOfDocument, offset: state.selectionExtent) {
                field.selectedTextRange = field.textRange(from: start, to: end)
            }
        }
    }

    private func handleTextChange(_ text: String) {
        // Don't send updates during programmatic text changes (e.g., setEditingState)
        // This prevents unnecessary round-trips that trigger form validation
        guard !suppressCallback else { return }

        // Get selection
        var selBase = text.count
        var selExtent = text.count

        if let field = textField, let range = field.selectedTextRange {
            selBase = field.offset(from: field.beginningOfDocument, to: range.start)
            selExtent = field.offset(from: field.beginningOfDocument, to: range.end)
        }

        editingState = TextEditingState(
            text: text,
            selectionBase: selBase,
            selectionExtent: selExtent,
            composingStart: -1,
            composingEnd: -1
        )

        // Send update to Go
        PlatformChannelManager.shared.sendEvent(
            channel: "drift/text_input",
            data: [
                "method": "updateEditingState",
                "connectionId": connectionId,
                "text": text,
                "selectionBase": selBase,
                "selectionExtent": selExtent,
                "composingStart": -1,
                "composingEnd": -1
            ]
        )
    }

    fileprivate func handleReturnKey() {
        // Notify Go of the action
        let action: Int
        var shouldDismiss = false

        switch config.returnKeyType {
        case .done:
            action = 1 // TextInputActionDone
            shouldDismiss = true
        case .go:
            action = 2 // TextInputActionGo
            shouldDismiss = true
        case .next:
            action = 3 // TextInputActionNext
        case .search:
            action = 5 // TextInputActionSearch
            shouldDismiss = true
        case .send:
            action = 6 // TextInputActionSend
            shouldDismiss = true
        default:
            action = 7 // TextInputActionNewline
        }

        // For actions that should dismiss, resign immediately rather than
        // waiting for the round-trip to Go and back
        if shouldDismiss {
            textField?.resignFirstResponder()
        }

        PlatformChannelManager.shared.sendEvent(
            channel: "drift/text_input",
            data: [
                "method": "performAction",
                "connectionId": connectionId,
                "action": action
            ]
        )
    }
}

// MARK: - UITextFieldDelegate

extension TextInputConnection: UITextFieldDelegate {
    func textFieldShouldReturn(_ textField: UITextField) -> Bool {
        handleReturnKey()
        return false
    }

    func textFieldDidEndEditing(_ textField: UITextField) {
        PlatformChannelManager.shared.sendEvent(
            channel: "drift/text_input",
            data: [
                "method": "connectionClosed",
                "connectionId": connectionId
            ]
        )
    }
}

// MARK: - Hidden Text Field

/// A hidden UITextField that captures keyboard input.
class HiddenTextField: UITextField {
    var connectionId: Int = 0
    var onTextChange: ((String) -> Void)?

    override init(frame: CGRect) {
        super.init(frame: frame)
        setup()
    }

    required init?(coder: NSCoder) {
        super.init(coder: coder)
        setup()
    }

    private func setup() {
        // Make the text field invisible but functional
        alpha = 0.01
        backgroundColor = .clear

        // Add target for text changes
        addTarget(self, action: #selector(textDidChange), for: .editingChanged)
    }

    @objc private func textDidChange() {
        onTextChange?(text ?? "")
    }

    // Override to ensure the field can become first responder even when hidden
    override var canBecomeFirstResponder: Bool {
        return true
    }
}

// MARK: - Keyboard Type Mapping

extension UIKeyboardType {
    init(rawValue: Int) {
        switch rawValue {
        case 0: self = .default
        case 1: self = .numberPad
        case 2: self = .phonePad
        case 3: self = .emailAddress
        case 4: self = .URL
        case 5: self = .default // Password (handled by secure entry)
        case 6: self = .default // Multiline
        default: self = .default
        }
    }
}

// MARK: - Return Key Type Mapping

extension UIReturnKeyType {
    init(rawValue: Int) {
        switch rawValue {
        case 0: self = .default
        case 1: self = .done
        case 2: self = .go
        case 3: self = .next
        case 4: self = .default // Previous (not available on iOS, use default)
        case 5: self = .search
        case 6: self = .send
        case 7: self = .default // Newline
        default: self = .default
        }
    }
}

