<script lang="ts">
    import type { Completion, CompletionContext, CompletionResult } from "@codemirror/autocomplete";
    import { acceptCompletion, autocompletion, closeCompletion, completionKeymap } from "@codemirror/autocomplete";
    import { defaultKeymap, history, historyKeymap, insertNewlineAndIndent } from "@codemirror/commands";
    import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
    import { HighlightStyle, bracketMatching, syntaxHighlighting, syntaxTree } from "@codemirror/language";
    import {
        type ChangeSpec,
        Compartment,
        EditorSelection,
        EditorState,
        type Extension,
        RangeSet,
    } from "@codemirror/state";
    import {
        Decoration,
        type DecorationSet,
        EditorView,
        ViewPlugin,
        ViewUpdate,
        WidgetType,
        drawSelection,
        dropCursor,
        highlightSpecialChars,
        keymap,
        placeholder,
    } from "@codemirror/view";
    import { tags } from "@lezer/highlight";
    import { type MarkdownConfig, Emoji as MarkdownEmoji, Strikethrough } from "@lezer/markdown";
    import type { CustomEmoji, Emoji, EmojiClickEvent } from "emoji-picker-element/shared";
    import { createEventDispatcher, onDestroy } from "svelte";
    import { Moon } from "svelte-loading-spinners";
    import { link } from "svelte-navigator";
    import BlockedUsers from "./BlockedUsers.svelte";
    import ChatComposeAreaAttachmentButton from "./ChatComposeAreaAttachmentButton.svelte";
    import ChatMediaPicker from "./ChatMediaPicker.svelte";
    import ChatReplyingBanner from "./ChatReplyingBanner.svelte";
    import { apiClient } from "./api_client";
    import { emojiDatabase } from "./chat_utils";
    import { closeBrackets, closeBracketsKeymap } from "./closebrackets";
    import GifMessagePreview from "./gifpicker/GifMessagePreview.svelte";
    import { openModal } from "./modal/modal";
    import { ChatGifSearchResult, ChatMessage, PermissionLevel, PointsInfoResponse } from "./proto/jungletv_pb";
    import {
        autoCloseBrackets,
        autoCloseMediaPickerOnInsert,
        autoCloseMediaPickerOnSend,
        chatEmotesAsCustomEmoji,
        chatMessageDraft,
        chatMessageDraftSelectionJSON,
        chatMessageDraftTenorGif,
        convertEmoticons,
        currentSubscription,
        darkMode,
        permissionLevel,
    } from "./stores";
    import ErrorMessage from "./uielements/ErrorMessage.svelte";
    import PointsIcon from "./uielements/PointsIcon.svelte";
    import WarningMessage from "./uielements/WarningMessage.svelte";
    import {
        codeMirrorHighlightStyle,
        emoteURLFromID,
        openPopout,
        parseUserMessageMarkdown,
        setNickname,
    } from "./utils";

    export let replyingToMessage: ChatMessage;
    export let hasBlockedMessages: boolean;

    let sendErrorMessage: "" | "insufficient-points" | "emotes-need-subscription" | "rate-limited" | "generic" = "";
    let customSendErrorMessage = "";
    let editorContainer: HTMLElement;
    let editorView: EditorView;

    let canSend = false;
    let showMediaPicker = false;

    $: canSend = $chatMessageDraft !== "" || typeof $chatMessageDraftTenorGif !== "undefined";

    let showedGuidelinesChatWarning = localStorage.getItem("showedGuidelinesChatWarning") == "true";

    const dispatch = createEventDispatcher();

    const themeCompartment = new Compartment();
    const highlightCompartment = new Compartment();
    const closeBracketsCompartment = new Compartment();

    const darkModeUnsubscribe = darkMode.subscribe((dm) => {
        if (typeof editorView !== "undefined") {
            editorView.dispatch({
                effects: [
                    themeCompartment.reconfigure(theme(dm)),
                    highlightCompartment.reconfigure(highlightStyle($permissionLevel == PermissionLevel.ADMIN, dm)),
                ],
            });
        }
    });
    onDestroy(darkModeUnsubscribe);

    const permissionLevelUnsubscribe = permissionLevel.subscribe((permLevel) => {
        if (typeof editorView !== "undefined") {
            editorView.dispatch({
                effects: highlightCompartment.reconfigure(
                    highlightStyle(permLevel == PermissionLevel.ADMIN, $darkMode)
                ),
            });
        }
    });
    onDestroy(permissionLevelUnsubscribe);

    const autoCloseBracketsUnsubscribe = autoCloseBrackets.subscribe((autoClose) => {
        if (typeof editorView !== "undefined") {
            editorView.dispatch({
                effects: closeBracketsCompartment.reconfigure(buildCloseBrackets(autoClose)),
            });
        }
    });
    onDestroy(autoCloseBracketsUnsubscribe);

    $: {
        if (typeof replyingToMessage !== "undefined" && typeof editorView !== "undefined") {
            editorView.focus();
        }
    }

    $: emojiDatabase.customEmoji = $chatEmotesAsCustomEmoji;

    function limitMaxLength(maxLength: number): Extension {
        return EditorState.changeFilter.of((tr): boolean | readonly number[] => {
            return tr.newDoc.length <= maxLength;
        });
    }

    function commandCompletions(context: CompletionContext): CompletionResult | null {
        if (context.state.doc.lineAt(context.state.selection.main.head).number > 1) {
            return null;
        }
        let word = context.matchBefore(/^\/.*/);
        if ((word == null || word.from == word.to) && !context.explicit) return null;
        return {
            from: word == null ? 0 : word.from,
            options: [
                { label: "/nick", type: "method", detail: "nickname or nothing", info: "Change or clear nickname" },
                { label: "/lightsout", type: "method", info: "Toggle dark theme" },
                { label: "/popout", type: "method", info: "Open chat in a separate window" },
                { label: "/shrug", type: "text", info: "Inserts ¯\\_(ツ)_/¯", apply: "¯\\\\\\_(ツ)\\_/¯" },
                { label: "/tableflip", type: "text", info: "Inserts (╯°□°）╯︵ ┻━┻", apply: "(╯°□°）╯︵ ┻━┻" },
                { label: "/unflip", type: "text", info: "Inserts ┬─┬ ノ( ゜-゜ノ)", apply: "┬─┬ ノ( ゜-゜ノ)" },
                {
                    label: "/spoiler",
                    type: "method",
                    detail: "message",
                    info: "Marks your message as spoiler",
                    apply: "/spoiler ",
                },
            ],
            validFor: /^\/\w*$/,
        };
    }

    const createRegExp = (str, opts) => new RegExp(str.raw[0].replace(/\s/gm, ""), opts || "");

    //        brow eyes  nose   mouth
    const emoticonRegExp = createRegExp`
        (\s|\\|^)
        (?:
            (      [=:]  -{0,1} \)    )|
            (      [=:]  -{0,1} \(    )|
            (      [=:]  -{0,1} D     )|
            (      [=:]  -{0,1} [oO]  )|
            (      [=:]  -{0,1} \|    )|
            (      ;     -{0,1} \(    )|
            (      ;     -{0,1} \)    )|
            (      [=:], -{0,1} \(    )|
            (    > [=:]  -{0,1} \(    )|
            (      [=:]  -{0,1} P     )|
            (    , [=:]  -{0,1} \)    )|
            (      [=:]  -{0,1} [$sz] )|
            (      [=:]  -{0,1} @     )|
            (      [=:], -{0,1} [)D]  )|
            (    , [=:]  -{0,1} \(    )|
            (      [=:]  -{0,1} \*    )|
            ([oO0] [=:]  -{0,1} \)    )|
            (   \] [=:]  -{0,1} \)    )|
            (   \] [=:]  -{0,1} \(    )|
            (   <3       )|
            (   <[\\/]3  )
        )
        (\s|$)
        ${"gm"}`;
    const emoticonsPerMatchGroup = [
        "🙂",
        "😦",
        "😄",
        "😮",
        "😐",
        "😭",
        "😉",
        "😢",
        "😠",
        "😛",
        "😅",
        "😒",
        "😡",
        "😂",
        "😓",
        "😗",
        "😇",
        "😈",
        "👿",
        "❤️",
        "💔",
    ];

    function replaceEmojiShortcodes(): Extension {
        return EditorView.updateListener.of(async (viewUpdate) => {
            if (viewUpdate.docChanged) {
                let oldContents = viewUpdate.state.doc.toString();
                let matches = oldContents.matchAll(/([\\|<a|<e]{0,1}):([a-zA-Z0-9_\+\-]+):/gm);
                let changes: ChangeSpec[] = [];
                for (let match of matches) {
                    if (match[1]) {
                        continue;
                    }
                    let result = await emojiDatabase.getEmojiByShortcode(match[2]);
                    if (result == null) {
                        continue;
                    }
                    if ("unicode" in result) {
                        changes.push({ from: match.index, to: match.index + match[0].length, insert: result.unicode });
                    } else if ("url" in result) {
                        changes.push({
                            from: match.index,
                            to: match.index + match[0].length,
                            insert: emoteStringFromCustomEmoji(result),
                        });
                    }
                }
                if ($convertEmoticons) {
                    let emoticonMatches = oldContents.matchAll(emoticonRegExp);
                    for (let match of emoticonMatches) {
                        if (match[1] == "\\") {
                            continue;
                        }
                        if (match.index + match[0].length != viewUpdate.state.selection.main.head) {
                            // only perform replacement if the cursor is at the end of the emoticon
                            // this allows for typing `(this is a test :|)`, where | is the cursor,
                            // without the resulting :) turning into a emoji (because the cursor would not be at the end of the match)
                            continue;
                        }
                        let lastGroup = match[2 + emoticonsPerMatchGroup.length];
                        let emoji = "";
                        for (let i = 2; i < 2 + emoticonsPerMatchGroup.length; i++) {
                            if (!match[i]) {
                                continue;
                            }
                            if (
                                (i == 5 || i == 13) &&
                                !lastGroup &&
                                (match[i].indexOf("s") >= 0 || match[i].indexOf("o") >= 0 || match[i].indexOf("z") >= 0)
                            ) {
                                // instant replacement of :s, :z and :o make it hard to type emoji shortcodes
                                continue;
                            }
                            emoji = emoticonsPerMatchGroup[i - 2];
                        }

                        if (emoji != "") {
                            changes.push({
                                from: match.index,
                                to: match.index + match[0].length,
                                insert: (match[1] ? match[1] : "") + emoji + (lastGroup ? lastGroup : ""),
                            });
                        }
                    }
                }
                if (changes.length > 0) {
                    viewUpdate.view.dispatch({
                        changes: changes,
                    });
                }
            }
        });
    }

    async function emojiCompletions(context: CompletionContext): Promise<CompletionResult | null> {
        let word = context.matchBefore(/(\\{0,1}):([a-zA-Z0-9_\+\-]+)/gm);
        if (word === null || word.to - word.from < 2 || word.text.length < 1 || word.text.startsWith("\\")) {
            return null;
        }
        let partialShortcode = word.text.substring(1);
        let emojiResults = await searchEmoji(partialShortcode, 5);
        let options: Completion[] = [];
        for (let result of emojiResults) {
            if ("unicode" in result) {
                options.push({
                    label: ":" + shortcodeMatchingPrefix(result.shortcodes, partialShortcode) + ":",
                    type: "emoji",
                    apply: result.unicode + " ",
                });
            } else if ("url" in result) {
                options.push({
                    label: ":" + shortcodeMatchingPrefix(result.shortcodes, partialShortcode) + ":",
                    type: "emote",
                    apply: emoteStringFromCustomEmoji(result as CustomEmoji) + " ",
                });
            }
        }
        return {
            from: word.from,
            options: options,
            filter: false,
        };
    }

    function shortcodeMatchingPrefix(shortcodes: string[], prefix: string): string {
        for (const shortcode of shortcodes) {
            if (shortcode.startsWith(prefix)) {
                return shortcode;
            }
        }
        return shortcodes[0];
    }

    async function searchEmoji(searchText: string, numResults: number): Promise<Emoji[]> {
        let emojis = await emojiDatabase.getEmojiBySearchQuery(searchText);

        let shortcode = searchText;
        if (searchText.endsWith(":")) {
            // exact shortcode search
            shortcode = searchText.substring(0, searchText.length - 1).toLowerCase();
            emojis = emojis.filter((_) => _.shortcodes.includes(shortcode));
        }
        if (emojis.findIndex((e) => e.shortcodes.includes(shortcode)) < 0) {
            // sometimes getEmojiBySearchQuery does not find the exact match for short queries
            // e.g. :m won't bring up the :m: emoji
            let exactMatch = await emojiDatabase.getEmojiByShortcode(shortcode);
            if (exactMatch != null) {
                emojis.push(exactMatch);
            }
        }

        // prefer emojis whose beginning of first shortcode matches exactly the searchText
        // this improves visual/behavior consistency
        let numMoved = 0;
        for (let i = emojis.length - 1; i >= numMoved; i--) {
            if (emojis[i].shortcodes[0].startsWith(searchText)) {
                emojis.unshift(emojis[i]);
                i++;
                emojis.splice(i, 1);
                numMoved++;
            }
        }
        return emojis.slice(0, numResults);
    }

    function addEmojiToAutocompleteOptions(completion: Completion, state: EditorState): Node | null {
        if (completion.type !== "emoji" || typeof completion.apply !== "string") {
            return null;
        }
        let node = document.createElement("div");
        node.innerText = completion.apply;
        node.classList.add("cm-completionEmoji");
        return node;
    }

    const emoteRegExp = /^<([ae])(:[a-zA-Z0-9_]+){0,1}:([0-9]{1,20})(\/{0,1})>/;

    function addEmoteToAutocompleteOptions(completion: Completion, state: EditorState): Node | null {
        if (completion.type !== "emote" || typeof completion.apply !== "string") {
            return null;
        }
        let node = document.createElement("div");
        let img = document.createElement("img");

        let match = completion.apply.match(emoteRegExp);
        img.src = emoteURLFromID(match[3].trim(), match[1].trim() == "a");
        node.appendChild(img);
        node.classList.add("cm-completionEmoji");
        return node;
    }

    const Emote: MarkdownConfig = {
        defineNodes: ["Emote"],
        parseInline: [
            {
                name: "Emote",
                parse(cx, next, pos) {
                    let match: RegExpMatchArray | null;
                    if (next != 60 /* '<' */ || !(match = emoteRegExp.exec(cx.slice(pos, cx.end)))) return -1;
                    return cx.addElement(cx.elt("Emote", pos, pos + match[0].length));
                },
            },
        ],
    };

    class EmoteWidget extends WidgetType {
        constructor(
            readonly originalText: string,
            readonly id: string,
            readonly shortcode: string,
            readonly animated: boolean
        ) {
            super();
        }

        eq(other: EmoteWidget) {
            return other.id == this.id;
        }

        toDOM() {
            let wrap = document.createElement("span");
            wrap.setAttribute("aria-hidden", "true");
            let img = wrap.appendChild(document.createElement("img"));
            img.addEventListener("error", () => {
                wrap.removeChild(img);
                wrap.style.display = "inline-block";
                wrap.style.fontSize = "65%";
                wrap.style.color = "red";
                wrap.style.lineHeight = "90%";
                wrap.style.marginTop = "-0.25rem";
                wrap.innerHTML = "invalid<br>emote";
            });
            img.src = emoteURLFromID(this.id, this.animated);
            img.alt = this.shortcode ? ":" + this.shortcode + ":" : "";
            img.title = this.shortcode ? ":" + this.shortcode + ":" : "";
            img.classList.add("markdown-emote");
            img.style.display = "inline";
            img.style.marginTop = "-0.25rem";
            return wrap;
        }
    }

    const emotePlugin = ViewPlugin.fromClass(
        class {
            decorations: DecorationSet;
            atomicRanges: RangeSet<any>;

            constructor(view: EditorView) {
                this.decorations = this.createEmoteReplacementWidgets(view);
            }

            createEmoteReplacementWidgets(view: EditorView) {
                let widgets = [];
                for (let { from, to } of view.visibleRanges) {
                    syntaxTree(view.state).iterate({
                        from,
                        to,
                        enter: (node) => {
                            if (node.type.name == "Emote") {
                                let match = view.state.doc.sliceString(node.from, node.to).match(emoteRegExp);
                                let deco = Decoration.replace({
                                    widget: new EmoteWidget(
                                        match[0],
                                        match[3],
                                        match[2]?.substring(1),
                                        match[1] == "a"
                                    ),
                                });
                                widgets.push(deco.range(node.from, node.to));
                            }
                        },
                    });
                }
                return Decoration.set(widgets);
            }

            update(update: ViewUpdate) {
                if (update.docChanged || update.viewportChanged)
                    this.decorations = this.createEmoteReplacementWidgets(update.view);
            }
        },
        {
            decorations: (v) => v.decorations,
            provide: (plugin) =>
                EditorView.atomicRanges.of((view) => {
                    let value = view.plugin(plugin);
                    return value ? value.decorations : Decoration.none;
                }),
        }
    );

    function theme(darkMode: boolean): Extension {
        return EditorView.theme(
            {
                "&.cm-editor": {
                    "max-height": "128px",
                },
                ".cm-scroller": {
                    "font-family": "inherit",
                    "line-height": "inherit",
                },
                "&.cm-editor.cm-focused": {
                    outline: "2px solid transparent",
                    "outline-offset": "2px",
                },
                ".cm-tooltip.cm-tooltip-autocomplete > ul": {
                    "max-height": "200px",
                    "font-family": "inherit",
                    padding: "8px",
                },
                ".cm-tooltip.cm-tooltip-autocomplete > ul > li": {
                    "font-family": "inherit",
                    "font-size": "1rem",
                    "line-height": "1.5rem",
                    padding: "3px 8px 3px 2px",
                    "text-color": darkMode ? "white" : "black",
                    "border-radius": "2px",
                },
                ".cm-completionIcon": {
                    "padding-right": "22px",
                    "font-size": "125%",
                },
                ".cm-completionIcon.cm-completionIcon-emoji": {
                    display: "none",
                },
                ".cm-completionIcon.cm-completionIcon-emote": {
                    display: "none",
                },
                ".cm-completionEmoji": {
                    display: "inline-block",
                    "text-align": "center",
                    "min-width": "2.1rem",
                    "padding-right": "0.3rem",
                    "vertical-align": "middle",
                },
                ".cm-completionEmoji > img": {
                    display: "inline",
                    height: "1.375em",
                    width: "1.375em",
                    "min-width": "1.375em",
                    "object-fit": "contain",
                    "margin-top": "-0.25rem",
                },
                ".cm-tooltip-autocomplete ul li[aria-selected]": {
                    "background-color": darkMode ? "rgb(75,85,99)" : "rgb(156,163,175)",
                    "text-color": darkMode ? "white" : "black",
                },
                ".cm-tooltip": {
                    background: darkMode ? "rgb(31,41,55)" : "rgb(229,231,235)",
                    "border-radius": "2px",
                    "border-width": "1px",
                    "border-color": darkMode ? "rgb(75,85,99)" : "rgb(156,163,175)",
                },
                "& .cm-cursor": {
                    "border-left-color": darkMode ? "#FBBF24" : "#B45309",
                },
                "& .cm-selectionBackground": {
                    "background-color": darkMode ? "#4C1D95" : "#DDD6FE",
                },
                "&.cm-focused .cm-selectionBackground": {
                    "background-color": darkMode ? "#5B21B6" : "#C4B5FD",
                },
            },
            {
                dark: darkMode,
            }
        );
    }

    function highlightStyle(fullMarkdown: boolean, darkMode: boolean): Extension {
        if (fullMarkdown) {
            return codeMirrorHighlightStyle(darkMode);
        }

        return syntaxHighlighting(
            HighlightStyle.define([
                { tag: tags.emphasis, fontStyle: "italic" },
                { tag: tags.strong, fontWeight: "bold" },
                { tag: tags.strikethrough, textDecoration: "line-through" },
                { tag: tags.monospace, fontFamily: "monospace", fontSize: "110%" },
                { tag: tags.character, color: "#a11" }, // Used by emoji shortcodes that aren't matched
            ])
        );
    }

    function buildCloseBrackets(autocloseBrackets: boolean): Extension {
        if (!autocloseBrackets) {
            return [];
        }
        return [closeBrackets(), keymap.of(closeBracketsKeymap)];
    }

    function setupEditor() {
        let initialSelection: EditorSelection;
        const selectionJSON = $chatMessageDraftSelectionJSON;
        if (selectionJSON != "") {
            initialSelection = EditorSelection.fromJSON(JSON.parse(selectionJSON));
        }
        editorView = new EditorView({
            state: EditorState.create({
                doc: $chatMessageDraft,
                selection: initialSelection,
                extensions: [
                    EditorView.updateListener.of((viewUpdate) => {
                        if (viewUpdate.docChanged) {
                            $chatMessageDraft = viewUpdate.state.doc.toString();
                        }
                        $chatMessageDraftSelectionJSON = JSON.stringify(viewUpdate.state.selection.toJSON());
                    }),
                    highlightSpecialChars(),
                    history(),
                    drawSelection(),
                    dropCursor(),
                    bracketMatching(),
                    closeBracketsCompartment.of(buildCloseBrackets($autoCloseBrackets)),
                    autocompletion({
                        override: [commandCompletions, emojiCompletions],
                        aboveCursor: true,
                        addToOptions: [
                            {
                                render: addEmojiToAutocompleteOptions,
                                position: 21,
                            },
                            {
                                render: addEmoteToAutocompleteOptions,
                                position: 21,
                            },
                        ],
                    }),
                    replaceEmojiShortcodes(),
                    highlightCompartment.of(highlightStyle($permissionLevel == PermissionLevel.ADMIN, $darkMode)),
                    keymap.of([
                        {
                            key: "Enter",
                            run: (): boolean => {
                                sendMessage(true);
                                return true;
                            },
                            shift: insertNewlineAndIndent,
                        },
                        ...defaultKeymap,
                        ...historyKeymap,
                        ...completionKeymap,
                        {
                            key: "Mod-Enter",
                            run: insertNewlineAndIndent,
                        },
                        {
                            key: "Tab",
                            run: acceptCompletion,
                        },
                    ]),
                    markdown({
                        extensions: [Strikethrough, MarkdownEmoji, Emote],
                        base: markdownLanguage,
                    }),
                    markdownLanguage.data.of({
                        closeBrackets: {
                            // note: we're using our own version of the closeBrackets extension
                            brackets: ["(", "[", "{", '"', "_", "*", "`"],
                            before: ")]}'\";>",
                            notAfter: ":-=", // prevents :( becoming :()
                        },
                    }),
                    EditorView.lineWrapping,
                    EditorView.contentAttributes.of({ enterKeyHint: "send", spellcheck: "true", autocorrect: "on" }),
                    emotePlugin,
                    placeholder("Say something..."),
                    limitMaxLength(512),
                    themeCompartment.of(theme($darkMode)),
                ],
            }),
            parent: editorContainer,
            root: editorContainer.getRootNode() as ShadowRoot,
        });
        editorView.focus();
        onDestroy(() => {
            editorView.destroy();
        });
    }

    $: {
        // reactive block to trigger editor initialization once editorContainer is bound
        if (typeof editorContainer !== "undefined" && typeof editorView === "undefined") {
            setupEditor();
        }
    }

    function updateEditorContents(newContents: string) {
        if (typeof editorView !== "undefined") {
            let curContents = editorView.state.doc.toString();
            if (newContents != curContents) {
                editorView.dispatch({
                    changes: { from: 0, to: curContents.length, insert: newContents },
                });
            }
        }
    }

    // reactive block to update the editor contents when composedMessage is updated
    $: updateEditorContents($chatMessageDraft);

    async function sendMessageFromEvent(event: Event) {
        await sendMessage(event.isTrusted);
    }

    async function sendMessage(isTrusted: boolean) {
        let msg = $chatMessageDraft.trim();
        let tenorGif = $chatMessageDraftTenorGif;
        if (msg == "" && typeof tenorGif === "undefined") {
            return;
        }

        $chatMessageDraft = "";
        $chatMessageDraftTenorGif = undefined;
        let refMsg = replyingToMessage;
        replyingToMessage = undefined;
        if ($autoCloseMediaPickerOnSend) {
            showMediaPicker = false;
        }

        if (msg == "/lightsout") {
            darkMode.update((v) => !v);
            return;
        } else if (msg == "/popout") {
            openPopout("chat");
            return;
        }
        if (msg.startsWith("/spoiler ")) {
            msg = "||" + msg.substring("/spoiler ".length) + "||";
        }
        try {
            if (msg.startsWith("/nick")) {
                let nickname = "";
                let parts = splitAtFirstSpace(msg);
                if (parts.length > 1) {
                    nickname = parts[1];
                }
                let [valid, errMsg] = await setNickname(nickname);
                if (!valid) {
                    customSendErrorMessage = errMsg;
                    sendErrorMessage = "generic";
                    setTimeout(() => (sendErrorMessage = ""), 10000);
                    return;
                }
            } else {
                dispatch("sentMessage");
                await apiClient.sendChatMessage(msg, isTrusted, refMsg, tenorGif?.getId());
            }
            sendErrorMessage = "";
        } catch (ex) {
            $chatMessageDraft = msg;
            replyingToMessage = refMsg;
            $chatMessageDraftTenorGif = tenorGif;
            if (ex.includes("insufficient points balance")) {
                sendErrorMessage = "insufficient-points";
            } else if (ex.includes("user is not allowed to use this emote")) {
                sendErrorMessage = "emotes-need-subscription";
            } else if (ex.includes("rate limit reached")) {
                sendErrorMessage = "rate-limited";
            } else {
                sendErrorMessage = "generic";
            }
            setTimeout(() => (sendErrorMessage = ""), 10000);
        }
        editorView.focus();
    }

    function dismissGuidelinesWarning() {
        showedGuidelinesChatWarning = true;
        localStorage.setItem("showedGuidelinesChatWarning", "true");
    }

    function splitAtFirstSpace(str) {
        var i = str.indexOf(" ");
        if (i > 0) {
            return [str.substring(0, i), str.substring(i + 1)];
        } else return [str];
    }

    function toggleMediaPicker() {
        if (showMediaPicker) {
            showMediaPicker = false;
            editorView.focus();
        } else {
            showMediaPicker = true;
            closeCompletion(editorView);
        }
    }

    function emoteStringFromCustomEmoji(emoji: CustomEmoji): string {
        let matches = emoji.url.match(/\/emotes\/([0-9]{1,20})\.(webp|gif)/);
        let emojiID = matches[1];
        let type = "";
        switch (matches[2]) {
            case "webp":
                type = "e";
                break;
            case "gif":
                type = "a";
                break;
        }
        return "<" + type + ":" + emoji.shortcodes[0] + ":" + emojiID + ">";
    }

    function onEmojiPicked(event: EmojiClickEvent) {
        if ($autoCloseMediaPickerOnInsert) {
            toggleMediaPicker();
        }
        if (event.detail.unicode) {
            editorView.dispatch(editorView.state.replaceSelection(event.detail.unicode + " "));
        } else {
            editorView.dispatch(
                editorView.state.replaceSelection(emoteStringFromCustomEmoji(event.detail.emoji as CustomEmoji) + " ")
            );
        }
        editorView.focus();
    }

    function onGifPicked(event: CustomEvent<ChatGifSearchResult>) {
        if ($autoCloseMediaPickerOnInsert) {
            toggleMediaPicker();
        }
        editorView.focus();
        $chatMessageDraftTenorGif = event.detail;
    }

    function openBlockedUserManagement() {
        openModal({
            component: BlockedUsers,
            options: {
                closeButton: true,
                closeOnEsc: true,
                closeOnOuterClick: true,
                styleContent: {
                    padding: "0",
                },
            },
        });
    }

    function removeTenorGifAttachment() {
        $chatMessageDraftTenorGif = undefined;
        editorView.focus();
    }

    async function pointsPromise(): Promise<PointsInfoResponse> {
        let response = await apiClient.pointsInfo();
        $currentSubscription = response.getCurrentSubscription();
        return response;
    }
</script>

{#if showMediaPicker}
    <ChatMediaPicker on:emoji-click={onEmojiPicked} on:gifSelected={onGifPicked} on:closePicker={toggleMediaPicker} />
{/if}
{#if sendErrorMessage != ""}
    <div class="px-2 pb-2 text-xs mt-2">
        <ErrorMessage>
            {#if sendErrorMessage == "insufficient-points"}
                You don't have sufficient points to send this message. <a href="/points" use:link>More information</a>
            {:else if sendErrorMessage == "emotes-need-subscription"}
                To use the emotes in your message,
                <a href="/points#nice" use:link>subscribe to <span class="font-semibold">JungleTV Nice</span></a>.
            {:else if sendErrorMessage == "rate-limited"}
                You're going too fast. Slow down.
            {:else if sendErrorMessage == "generic" && customSendErrorMessage == ""}
                Failed to send your message. Please try again.
            {:else}
                {customSendErrorMessage}
            {/if}
        </ErrorMessage>
    </div>
{/if}
{#if !showedGuidelinesChatWarning}
    <div class="px-2 pb-2 text-xs mt-2">
        <WarningMessage>
            Before participating in chat, make sure to read the
            <a use:link href="/guidelines" class="dark:text-blue-600">community guidelines</a>.
            <br />
            <button
                type="button"
                class="font-semibold float-right dark:text-blue-600"
                on:click={dismissGuidelinesWarning}>I read the guidelines and will respect them</button
            >
        </WarningMessage>
    </div>
{/if}
{#if hasBlockedMessages}
    <div class="px-2 py-1 text-xs">
        Some messages were hidden.
        <button
            type="button"
            class="text-blue-600 dark:text-blue-400 cursor-pointer hover:underline"
            on:click={openBlockedUserManagement}
        >
            Manage blocked users
        </button>
    </div>
{/if}
{#if replyingToMessage !== undefined}
    <ChatReplyingBanner
        {replyingToMessage}
        on:clearReply={() => {
            replyingToMessage = undefined;
        }}
    >
        <svelte:fragment slot="message-content">
            {@html parseUserMessageMarkdown(replyingToMessage.getUserMessage().getContent(), false)[0]}
        </svelte:fragment>
    </ChatReplyingBanner>
{/if}
<div class="flex flex-row relative">
    <div class="grow">
        <div class="p-1 focus:outline-none" bind:this={editorContainer} />
        {#if typeof $chatMessageDraftTenorGif !== "undefined"}
            <div class="flex flex-row items-center space-x-2 px-2">
                <GifMessagePreview gif={$chatMessageDraftTenorGif} />
                <div class="text-gray-600 dark:text-gray-400 text-base">
                    <div>
                        <PointsIcon />
                        <span class="font-semibold">{-$chatMessageDraftTenorGif.getPointsCost()}</span>
                        {#await pointsPromise()}
                            <span class="text-xs inline-flex">
                                <span class="mr-1">/</span>
                                <Moon size="15" color={$darkMode ? "#FFFFFF" : "#444444"} unit="px" duration="2s" />
                            </span>
                        {:then response}
                            <span class="text-xs">
                                /
                                <span
                                    class={response.getBalance() < $chatMessageDraftTenorGif.getPointsCost()
                                        ? "text-red-700"
                                        : ""}
                                >
                                    {response.getBalance()}
                                </span>
                            </span>
                        {/await}
                    </div>
                    <div><a use:link href="/points" class="text-sm">More information</a></div>
                    <div>
                        <button
                            class="text-blue-600 dark:text-blue-400 hover:underline"
                            type="button"
                            on:click={removeTenorGifAttachment}
                        >
                            Remove
                        </button>
                    </div>
                </div>
            </div>
        {/if}
    </div>
    <ChatComposeAreaAttachmentButton on:click={toggleMediaPicker} />

    <button
        type="submit"
        title="Send message"
        class="{!canSend ? 'text-gray-400 dark:text-gray-600' : 'text-purple-700 dark:text-purple-500'}
        min-h-full w-8 p-2 shadow-md bg-gray-100 dark:bg-gray-800 dark:hover:bg-gray-700 hover:bg-gray-200 ease-linear transition-all duration-150"
        on:click={sendMessageFromEvent}
    >
        <i class="fas fa-paper-plane" />
    </button>
</div>
