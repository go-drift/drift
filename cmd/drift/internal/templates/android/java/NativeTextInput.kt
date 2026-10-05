/**
 * NativeTextInput.kt
 * Provides native text input views embedded in Drift UI with Skia chrome.
 */
package {{.PackageName}}

import com.drift.runner.DriftPlatformView
import android.content.Context
import android.graphics.Color
import android.graphics.Typeface
import android.text.Editable
import android.text.InputType
import android.text.TextWatcher
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.view.inputmethod.EditorInfo
import android.view.inputmethod.InputMethodManager
import android.widget.EditText
import android.widget.FrameLayout

/**
 * Platform view container for native text input.
 */
class NativeTextInputContainer(
    context: Context,
    val viewId: Int,
    params: Map<String, Any?>
) : DriftPlatformView {

    override val view: View
    private val editText: EditText
    private var config: TextInputViewConfig
    private var suppressCallback: Boolean = false

    init {
        config = TextInputViewConfig(params)

        editText = EditText(context).apply {
            // Transparent background - Skia draws the chrome
            background = null
            setBackgroundColor(Color.TRANSPARENT)

            // Apply config
            applyConfig(config)

            // Text change listener
            addTextChangedListener(object : TextWatcher {
                override fun beforeTextChanged(s: CharSequence?, start: Int, count: Int, after: Int) {}
                override fun onTextChanged(s: CharSequence?, start: Int, before: Int, count: Int) {}
                override fun afterTextChanged(s: Editable?) {
                    if (!suppressCallback) {
                        sendTextChanged()
                    }
                }
            })

            // IME action listener
            setOnEditorActionListener { _, actionId, _ ->
                sendAction(actionIdToAction(actionId))
                when (actionId) {
                    // Dismiss keyboard for completion actions
                    EditorInfo.IME_ACTION_DONE,
                    EditorInfo.IME_ACTION_GO,
                    EditorInfo.IME_ACTION_SEARCH,
                    EditorInfo.IME_ACTION_SEND -> {
                        clearFocus()
                        hideKeyboard()
                        true
                    }
                    // Navigation actions should be consumed even in multiline
                    EditorInfo.IME_ACTION_NEXT,
                    EditorInfo.IME_ACTION_PREVIOUS -> {
                        true
                    }
                    // For other actions (e.g., unspecified/none), allow newline in multiline
                    else -> {
                        !config.multiline
                    }
                }
            }

            // Focus change listener
            setOnFocusChangeListener { _, hasFocus ->
                sendFocusChanged(hasFocus)
            }

            layoutParams = FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.MATCH_PARENT,
                FrameLayout.LayoutParams.MATCH_PARENT
            )
        }

        view = editText

        // Apply initial text if provided
        (params["text"] as? String)?.let { setText(it) }
    }

    override fun handle(method: String, args: Map<String, Any?>) {
        when (method) {
            "setText" -> setText(args["text"] as? String ?: "")
            "setSelection" -> setSelection(
                (args["selectionBase"] as? Number)?.toInt() ?: 0,
                (args["selectionExtent"] as? Number)?.toInt() ?: 0,
            )
            "setValue" -> {
                val text = args["text"] as? String ?: ""
                setValue(
                    text,
                    (args["selectionBase"] as? Number)?.toInt() ?: text.length,
                    (args["selectionExtent"] as? Number)?.toInt() ?: text.length,
                )
            }
            "focus" -> focus()
            "blur" -> blur()
            "updateConfig" -> updateConfig(args)
            else -> throw unknownViewMethod(method)
        }
    }

    override fun dispose() {
        hideKeyboard()
        editText.clearFocus()
    }

    // MARK: - View Methods

    fun setText(text: String) {
        suppressCallback = true
        editText.setText(text)
        suppressCallback = false
    }

    fun setSelection(base: Int, extent: Int) {
        val safeBase = base.coerceIn(0, editText.text.length)
        val safeExtent = extent.coerceIn(0, editText.text.length)
        editText.setSelection(minOf(safeBase, safeExtent), maxOf(safeBase, safeExtent))
    }

    fun setValue(text: String, selectionBase: Int, selectionExtent: Int) {
        setText(text)
        setSelection(selectionBase, selectionExtent)
    }

    fun focus() {
        editText.requestFocus()
        showKeyboard()
    }

    fun blur() {
        editText.clearFocus()
        hideKeyboard()
    }

    fun updateConfig(params: Map<String, Any?>) {
        config = TextInputViewConfig(params)
        editText.applyConfig(config)
    }

    // MARK: - Event Sending

    private fun sendTextChanged() {
        val text = editText.text.toString()
        val selStart = editText.selectionStart
        val selEnd = editText.selectionEnd

        PlatformChannelManager.sendEvent(
            "drift/platform_views",
            mapOf(
                "method" to "onTextChanged",
                "viewId" to viewId,
                "text" to text,
                "selectionBase" to selStart,
                "selectionExtent" to selEnd
            )
        )
    }

    private fun sendAction(action: Int) {
        PlatformChannelManager.sendEvent(
            "drift/platform_views",
            mapOf(
                "method" to "onAction",
                "viewId" to viewId,
                "action" to action
            )
        )
    }

    private fun sendFocusChanged(focused: Boolean) {
        PlatformChannelManager.sendEvent(
            "drift/platform_views",
            mapOf(
                "method" to "onFocusChanged",
                "viewId" to viewId,
                "focused" to focused
            )
        )
    }

    // MARK: - Helpers

    private fun showKeyboard() {
        val imm = editText.context.getSystemService(Context.INPUT_METHOD_SERVICE) as InputMethodManager
        imm.showSoftInput(editText, InputMethodManager.SHOW_IMPLICIT)
    }

    private fun hideKeyboard() {
        val imm = editText.context.getSystemService(Context.INPUT_METHOD_SERVICE) as InputMethodManager
        imm.hideSoftInputFromWindow(editText.windowToken, 0)
    }

    private fun actionIdToAction(actionId: Int): Int {
        return when (actionId) {
            EditorInfo.IME_ACTION_DONE -> 1
            EditorInfo.IME_ACTION_GO -> 2
            EditorInfo.IME_ACTION_NEXT -> 3
            EditorInfo.IME_ACTION_PREVIOUS -> 4
            EditorInfo.IME_ACTION_SEARCH -> 5
            EditorInfo.IME_ACTION_SEND -> 6
            else -> 7 // newline
        }
    }

    private fun EditText.applyConfig(config: TextInputViewConfig) {
        // Font
        setTextSize(TypedValue.COMPLEX_UNIT_SP, config.fontSize)
        typeface = config.typeface

        // Colors
        setTextColor(config.textColor)
        setHintTextColor(config.placeholderColor)

        // Alignment
        gravity = config.gravity

        // Only set inputType when it changed. Redundant setInputType on password
        // fields re-applies PasswordTransformationMethod (Android skips the
        // short-circuit for password types), which disrupts cursor position.
        if (inputType != config.inputType) {
            inputType = config.inputType
        }

        // IME options
        imeOptions = config.imeOptions

        // Padding
        val density = resources.displayMetrics.density
        setPadding(
            (config.paddingLeft * density).toInt(),
            (config.paddingTop * density).toInt(),
            (config.paddingRight * density).toInt(),
            (config.paddingBottom * density).toInt()
        )

        // Placeholder
        hint = config.placeholder

        // Multiline
        if (config.multiline) {
            setSingleLine(false)
            if (config.maxLines > 0) {
                maxLines = config.maxLines
            }
        } else {
            setSingleLine(true)
        }
    }
}

/**
 * Configuration for native text input view.
 */
class TextInputViewConfig(params: Map<String, Any?>) {
    val fontFamily: String = params["fontFamily"] as? String ?: ""
    val fontSize: Float = (params["fontSize"] as? Number)?.toFloat() ?: 16f
    val fontWeight: Int = (params["fontWeight"] as? Number)?.toInt() ?: 400
    val textColor: Int
    val placeholderColor: Int
    val textAlignment: Int = (params["textAlignment"] as? Number)?.toInt() ?: 0
    val multiline: Boolean = params["multiline"] as? Boolean ?: false
    val maxLines: Int = (params["maxLines"] as? Number)?.toInt() ?: 0
    val obscure: Boolean = params["obscure"] as? Boolean ?: false
    val autocorrect: Boolean = params["autocorrect"] as? Boolean ?: true
    val keyboardType: Int = (params["keyboardType"] as? Number)?.toInt() ?: 0
    val inputAction: Int = (params["inputAction"] as? Number)?.toInt() ?: 1
    val capitalization: Int = (params["capitalization"] as? Number)?.toInt() ?: 3
    val paddingLeft: Float = (params["paddingLeft"] as? Number)?.toFloat() ?: 0f
    val paddingTop: Float = (params["paddingTop"] as? Number)?.toFloat() ?: 0f
    val paddingRight: Float = (params["paddingRight"] as? Number)?.toFloat() ?: 0f
    val paddingBottom: Float = (params["paddingBottom"] as? Number)?.toFloat() ?: 0f
    val placeholder: String = params["placeholder"] as? String ?: ""

    init {
        val textColorArg = params["textColor"]
        textColor = when (textColorArg) {
            is Number -> textColorArg.toInt()
            else -> Color.BLACK
        }

        val placeholderColorArg = params["placeholderColor"]
        placeholderColor = when (placeholderColorArg) {
            is Number -> placeholderColorArg.toInt()
            else -> Color.GRAY
        }
    }

    val typeface: Typeface
        get() {
            val style = when {
                fontWeight >= 700 -> Typeface.BOLD
                else -> Typeface.NORMAL
            }
            return if (fontFamily.isNotEmpty()) {
                try {
                    Typeface.create(fontFamily, style)
                } catch (_: Exception) {
                    Typeface.defaultFromStyle(style)
                }
            } else {
                Typeface.defaultFromStyle(style)
            }
        }

    val gravity: Int
        get() {
            val vertical = if (multiline) Gravity.TOP else Gravity.CENTER_VERTICAL
            return when (textAlignment) {
                1 -> Gravity.CENTER_HORIZONTAL or vertical
                2 -> Gravity.END or vertical
                else -> Gravity.START or vertical
            }
        }

    val inputType: Int
        get() {
            var type = when (keyboardType) {
                0 -> InputType.TYPE_CLASS_TEXT
                1 -> InputType.TYPE_CLASS_NUMBER
                2 -> InputType.TYPE_CLASS_PHONE
                3 -> InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_EMAIL_ADDRESS
                4 -> InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_URI
                5 -> InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_PASSWORD
                6 -> InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_FLAG_MULTI_LINE
                else -> InputType.TYPE_CLASS_TEXT
            }

            if (obscure) {
                type = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_PASSWORD
            }

            if (!autocorrect) {
                type = type or InputType.TYPE_TEXT_FLAG_NO_SUGGESTIONS
            }

            // Capitalization
            when (capitalization) {
                1 -> type = type or InputType.TYPE_TEXT_FLAG_CAP_CHARACTERS
                2 -> type = type or InputType.TYPE_TEXT_FLAG_CAP_WORDS
                3 -> type = type or InputType.TYPE_TEXT_FLAG_CAP_SENTENCES
            }

            if (multiline) {
                type = type or InputType.TYPE_TEXT_FLAG_MULTI_LINE
            }

            return type
        }

    val imeOptions: Int
        get() = when (inputAction) {
            0 -> EditorInfo.IME_ACTION_UNSPECIFIED
            1 -> EditorInfo.IME_ACTION_DONE
            2 -> EditorInfo.IME_ACTION_GO
            3 -> EditorInfo.IME_ACTION_NEXT
            4 -> EditorInfo.IME_ACTION_PREVIOUS
            5 -> EditorInfo.IME_ACTION_SEARCH
            6 -> EditorInfo.IME_ACTION_SEND
            7 -> EditorInfo.IME_ACTION_NONE // Newline
            else -> EditorInfo.IME_ACTION_DONE
        }
}
