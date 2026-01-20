/**
 * TextInput.kt
 * Provides native text input (IME) handling for the Drift framework.
 */
package {{.PackageName}}

import android.content.Context
import android.graphics.Color
import android.text.Editable
import android.text.InputType
import android.text.TextWatcher
import android.view.View
import android.view.ViewGroup
import android.view.inputmethod.EditorInfo
import android.view.inputmethod.InputMethodManager
import android.widget.EditText
import android.widget.FrameLayout

/**
 * Handles text input channel methods from Go.
 */
object TextInputHandler {
    private val connections = mutableMapOf<Int, TextInputConnection>()
    private var context: Context? = null
    private var hostView: ViewGroup? = null

    fun init(context: Context, hostView: ViewGroup) {
        this.context = context
        this.hostView = hostView
    }

    fun handle(method: String, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))

        return when (method) {
            "createConnection" -> createConnection(argsMap)
            "closeConnection" -> closeConnection(argsMap)
            "show" -> show(argsMap)
            "hide" -> hide(argsMap)
            "setEditingState" -> setEditingState(argsMap)
            else -> Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
    }

    private fun createConnection(args: Map<*, *>): Pair<Any?, Exception?> {
        val connectionId = (args["connectionId"] as? Number)?.toInt()
            ?: return Pair(null, IllegalArgumentException("Missing connectionId"))

        val keyboardType = (args["keyboardType"] as? Number)?.toInt() ?: 0
        val inputAction = (args["inputAction"] as? Number)?.toInt() ?: 0
        val autocorrect = args["autocorrect"] as? Boolean ?: true
        val obscure = args["obscure"] as? Boolean ?: false
        val capitalization = (args["capitalization"] as? Number)?.toInt() ?: 3

        val config = TextInputConfiguration(
            keyboardType = keyboardType,
            inputAction = inputAction,
            autocorrect = autocorrect,
            obscure = obscure,
            capitalization = capitalization
        )

        val ctx = context ?: return Pair(null, IllegalStateException("Context not initialized"))
        val host = hostView ?: return Pair(null, IllegalStateException("Host view not initialized"))

        val connection = TextInputConnection(ctx, host, connectionId, config)
        connections[connectionId] = connection

        return Pair(mapOf("created" to true), null)
    }

    private fun closeConnection(args: Map<*, *>): Pair<Any?, Exception?> {
        val connectionId = (args["connectionId"] as? Number)?.toInt() ?: return Pair(null, null)

        connections[connectionId]?.close()
        connections.remove(connectionId)

        return Pair(null, null)
    }

    private fun show(args: Map<*, *>): Pair<Any?, Exception?> {
        val connectionId = (args["connectionId"] as? Number)?.toInt() ?: return Pair(null, null)
        connections[connectionId]?.show()
        return Pair(null, null)
    }

    private fun hide(args: Map<*, *>): Pair<Any?, Exception?> {
        val connectionId = (args["connectionId"] as? Number)?.toInt() ?: return Pair(null, null)
        connections[connectionId]?.hide()
        return Pair(null, null)
    }

    private fun setEditingState(args: Map<*, *>): Pair<Any?, Exception?> {
        val connectionId = (args["connectionId"] as? Number)?.toInt() ?: return Pair(null, null)
        val connection = connections[connectionId] ?: return Pair(null, null)

        val text = args["text"] as? String ?: ""
        val selectionBase = (args["selectionBase"] as? Number)?.toInt() ?: text.length
        val selectionExtent = (args["selectionExtent"] as? Number)?.toInt() ?: text.length
        val composingStart = (args["composingStart"] as? Number)?.toInt() ?: -1
        val composingEnd = (args["composingEnd"] as? Number)?.toInt() ?: -1

        val state = TextEditingState(
            text = text,
            selectionBase = selectionBase,
            selectionExtent = selectionExtent,
            composingStart = composingStart,
            composingEnd = composingEnd
        )

        connection.setEditingState(state)
        return Pair(null, null)
    }
}

/**
 * Configuration for a text input connection.
 */
data class TextInputConfiguration(
    val keyboardType: Int,
    val inputAction: Int,
    val autocorrect: Boolean,
    val obscure: Boolean,
    val capitalization: Int
)

/**
 * Represents the current text editing state.
 */
data class TextEditingState(
    val text: String,
    val selectionBase: Int,
    val selectionExtent: Int,
    val composingStart: Int,
    val composingEnd: Int
)

/**
 * Manages a single text input connection with the keyboard.
 */
class TextInputConnection(
    private val context: Context,
    private val hostView: ViewGroup,
    private val connectionId: Int,
    private val config: TextInputConfiguration
) {
    private var editText: EditText? = null
    private var isUpdatingState = false

    fun show() {
        hostView.post {
            if (editText == null) {
                createEditText()
            }
            editText?.let { et ->
                et.inputType = getInputType()
                et.imeOptions = getImeOptions()
                et.requestFocus()
                val imm = context.getSystemService(Context.INPUT_METHOD_SERVICE) as InputMethodManager
                imm.showSoftInput(et, InputMethodManager.SHOW_IMPLICIT)
            }
        }
    }

    fun hide() {
        hostView.post {
            editText?.let { et ->
                val imm = context.getSystemService(Context.INPUT_METHOD_SERVICE) as InputMethodManager
                imm.hideSoftInputFromWindow(et.windowToken, 0)
                et.clearFocus()
            }
        }
    }

    fun close() {
        hostView.post {
            editText?.let { et ->
                val imm = context.getSystemService(Context.INPUT_METHOD_SERVICE) as InputMethodManager
                imm.hideSoftInputFromWindow(et.windowToken, 0)
                hostView.removeView(et)
            }
            editText = null
        }
    }

    fun setEditingState(state: TextEditingState) {
        hostView.post {
            editText?.let { et ->
                isUpdatingState = true
                et.setText(state.text)
                val start = state.selectionBase.coerceIn(0, state.text.length)
                val end = state.selectionExtent.coerceIn(0, state.text.length)
                et.setSelection(minOf(start, end), maxOf(start, end))
                isUpdatingState = false
            }
        }
    }

    private fun createEditText() {
        val et = EditText(context).apply {
            // Make it invisible but functional
            layoutParams = FrameLayout.LayoutParams(1, 1).apply {
                setMargins(-1000, -1000, 0, 0)
            }
            setBackgroundColor(Color.TRANSPARENT)
            setTextColor(Color.TRANSPARENT)

            // Configure input type based on config
            inputType = getInputType()
            imeOptions = getImeOptions()

            // Add text change listener
            addTextChangedListener(object : TextWatcher {
                override fun beforeTextChanged(s: CharSequence?, start: Int, count: Int, after: Int) {}
                override fun onTextChanged(s: CharSequence?, start: Int, before: Int, count: Int) {}
                override fun afterTextChanged(s: Editable?) {
                    if (!isUpdatingState) {
                        handleTextChanged()
                    }
                }
            })

            // Handle IME action
            setOnEditorActionListener { _, actionId, _ ->
                handleImeAction(actionId)
                true
            }

            // Handle focus change
            setOnFocusChangeListener { _, hasFocus ->
                if (!hasFocus) {
                    notifyConnectionClosed()
                }
            }
        }

        hostView.addView(et)
        editText = et
    }

    private fun getInputType(): Int {
        var type = when (config.keyboardType) {
            0 -> InputType.TYPE_CLASS_TEXT
            1 -> InputType.TYPE_CLASS_NUMBER
            2 -> InputType.TYPE_CLASS_PHONE
            3 -> InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_EMAIL_ADDRESS
            4 -> InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_URI
            5 -> InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_PASSWORD
            6 -> InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_FLAG_MULTI_LINE
            else -> InputType.TYPE_CLASS_TEXT
        }

        if (config.obscure) {
            type = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_PASSWORD
        }

        if (!config.autocorrect) {
            type = type or InputType.TYPE_TEXT_FLAG_NO_SUGGESTIONS
        }

        // Capitalization
        when (config.capitalization) {
            1 -> type = type or InputType.TYPE_TEXT_FLAG_CAP_CHARACTERS
            2 -> type = type or InputType.TYPE_TEXT_FLAG_CAP_WORDS
            3 -> type = type or InputType.TYPE_TEXT_FLAG_CAP_SENTENCES
        }

        return type
    }

    private fun getImeOptions(): Int {
        return when (config.inputAction) {
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

    private fun handleTextChanged() {
        val et = editText ?: return
        val text = et.text.toString()
        val selStart = et.selectionStart
        val selEnd = et.selectionEnd

        PlatformChannelManager.sendEvent(
            "drift/text_input",
            mapOf(
                "method" to "updateEditingState",
                "connectionId" to connectionId,
                "text" to text,
                "selectionBase" to selStart,
                "selectionExtent" to selEnd,
                "composingStart" to -1,
                "composingEnd" to -1
            )
        )
    }

    private fun handleImeAction(actionId: Int): Boolean {
        val action = when (actionId) {
            EditorInfo.IME_ACTION_DONE -> 1
            EditorInfo.IME_ACTION_GO -> 2
            EditorInfo.IME_ACTION_NEXT -> 3
            EditorInfo.IME_ACTION_PREVIOUS -> 4
            EditorInfo.IME_ACTION_SEARCH -> 5
            EditorInfo.IME_ACTION_SEND -> 6
            else -> 7 // Newline
        }

        PlatformChannelManager.sendEvent(
            "drift/text_input",
            mapOf(
                "method" to "performAction",
                "connectionId" to connectionId,
                "action" to action
            )
        )
        return true
    }

    private fun notifyConnectionClosed() {
        PlatformChannelManager.sendEvent(
            "drift/text_input",
            mapOf(
                "method" to "connectionClosed",
                "connectionId" to connectionId
            )
        )
    }
}
