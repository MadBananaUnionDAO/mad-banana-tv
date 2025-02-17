<script lang="ts">
    import { acceptCompletion, closeBracketsKeymap, completionKeymap } from "@codemirror/autocomplete";
    import { defaultKeymap, historyKeymap, indentWithTab } from "@codemirror/commands";
    import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
    import { foldKeymap } from "@codemirror/language";
    import { lintKeymap } from "@codemirror/lint";
    import { searchKeymap } from "@codemirror/search";
    import { Compartment, EditorState } from "@codemirror/state";
    import { EditorView, keymap } from "@codemirror/view";
    import { Emoji, Strikethrough } from "@lezer/markdown";
    import { basicSetup } from "codemirror";
    import { onDestroy } from "svelte";
    import watchMedia from "svelte-media";
    import { link } from "svelte-navigator";
    import { HSplitPane } from "svelte-split-pane";
    import { apiClient } from "../api_client";
    import { modalAlert } from "../modal/modal";
    import { Document } from "../proto/jungletv_pb";
    import { darkMode } from "../stores";
    import ButtonButton from "../uielements/ButtonButton.svelte";
    import { hrefButtonStyleClasses, parseCompleteMarkdown } from "../utils";
    import { editorHighlightStyle, editorTheme } from "./codeEditor";

    export let documentID = "";
    let content = "";
    let editing = false;

    async function fetchDocument(): Promise<Document> {
        try {
            let response = await apiClient.getDocument(documentID);
            content = response.getContent();
            editing = true;
            return response;
        } catch {
            content = "";
            editing = false;
            return new Document();
        }
    }

    async function save() {
        let document = new Document();
        document.setId(documentID);
        document.setContent(content);
        document.setFormat("markdown");
        await apiClient.updateDocument(document);
        await modalAlert("Document updated");
        editing = true;
    }

    async function triggerAnnouncementsNotification() {
        await apiClient.triggerAnnouncementsNotification();
        await modalAlert("Announcements notification triggered");
    }

    let editorContainer: HTMLElement;
    let editorView: EditorView;

    const themeCompartment = new Compartment();
    const highlightCompartment = new Compartment();

    const darkModeUnsubscribe = darkMode.subscribe((dm) => {
        if (typeof editorView !== "undefined") {
            editorView.dispatch({
                effects: [
                    themeCompartment.reconfigure(editorTheme(dm)),
                    highlightCompartment.reconfigure(editorHighlightStyle(dm)),
                ],
            });
        }
    });
    onDestroy(darkModeUnsubscribe);

    function setupEditor() {
        editorView = new EditorView({
            state: EditorState.create({
                doc: content,
                extensions: [
                    EditorView.updateListener.of((viewUpdate) => {
                        if (viewUpdate.docChanged) {
                            content = viewUpdate.state.doc.toString();
                        }
                    }),
                    basicSetup,
                    highlightCompartment.of(editorHighlightStyle($darkMode)),
                    keymap.of([
                        ...closeBracketsKeymap,
                        ...defaultKeymap,
                        ...searchKeymap,
                        ...historyKeymap,
                        ...foldKeymap,
                        ...completionKeymap,
                        ...lintKeymap,
                        {
                            key: "Tab",
                            run: acceptCompletion,
                        },
                        indentWithTab,
                        {
                            key: "Mod-s",
                            preventDefault: true,
                            run: (_): boolean => {
                                save();
                                return true;
                            },
                        },
                    ]),
                    markdown({
                        extensions: [Strikethrough, Emoji],
                        base: markdownLanguage,
                    }),
                    EditorView.lineWrapping,
                    themeCompartment.of(editorTheme($darkMode)),
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

    // reactive block to update the editor contents when content is updated
    $: updateEditorContents(content);

    let leftPaneSize = "50%";
    let rightPaneSize = "50%";

    function toggleEditorPreview() {
        if (leftPaneSize == "0%") {
            leftPaneSize = "100%";
            rightPaneSize = "0%";
        } else {
            leftPaneSize = "0%";
            rightPaneSize = "100%";
        }
    }
    const media = watchMedia({ large: "(min-width: 640px)" });
    let firstMedia = true;
    // make sure we don't attempt to even split the screen on narrow screens
    const mediaUnsubscribe = media.subscribe((obj) => {
        if (firstMedia) {
            firstMedia = false;
            if (!obj.large) {
                leftPaneSize = "100%";
                rightPaneSize = "0%";
            }
        }
    });
    onDestroy(mediaUnsubscribe);
</script>

<div class="grow mx-auto editor-container flex flex-col">
    <div class="flex flex-row flex-wrap space-x-2 bg-gray-50 dark:bg-gray-950">
        <a use:link href="/moderate" class="block {hrefButtonStyleClasses()}}">
            <i class="fas fa-arrow-left" />
        </a>
        <h1 class="text-lg block pt-1">
            <span class="hidden md:inline">{editing ? "Editing" : "Creating"} document</span>
            <span class="font-mono">{documentID}</span>
        </h1>
        <div class="grow" />
        <ButtonButton color="gray" extraClasses="block lg:hidden" on:click={toggleEditorPreview}>
            Toggle preview
        </ButtonButton>
        <div class="grow" />
        <ButtonButton type="submit" on:click={save} extraClasses="block">Save</ButtonButton>
        {#if documentID == "announcements"}
            <ButtonButton color="blue" on:click={triggerAnnouncementsNotification}>
                Trigger new announcement notification
            </ButtonButton>
        {/if}
    </div>

    <div class="overflow-hidden h-full">
        {#await fetchDocument()}
            <p>Loading document...</p>
        {:then}
            <HSplitPane {leftPaneSize} {rightPaneSize}>
                <div slot="left" class="h-full max-h-full relative" bind:this={editorContainer} />
                <div slot="right" class="h-full max-h-full px-6 pb-6 overflow-auto markdown-document">
                    {@html parseCompleteMarkdown(content)}
                </div>
            </HSplitPane>
        {/await}
    </div>
</div>

<style>
    .editor-container {
        width: 100%;
        height: calc(100vh - 4rem);
    }
</style>
