<script lang="ts">
    import { onMount } from "svelte";
    import { navigate } from "svelte-navigator";
    import { apiClient } from "./api_client";

    import EnqueueFailure from "./EnqueueFailure.svelte";
    import EnqueueMediaSelection from "./EnqueueMediaSelection.svelte";
    import EnqueuePayment from "./EnqueuePayment.svelte";
    import EnqueueRafflePromotion from "./EnqueueRafflePromotion.svelte";
    import EnqueueSuccess from "./EnqueueSuccess.svelte";
    import type { EnqueueMediaResponse, EnqueueMediaTicket, OngoingRaffleInfo } from "./proto/jungletv_pb";
    import type { MediaSelectionKind } from "./utils";

    let step = 0;
    let ticket: EnqueueMediaTicket;
    let mediaKind: MediaSelectionKind = "video";
    function onMediaSelected(event: CustomEvent<EnqueueMediaResponse>) {
        ticket = event.detail.getTicket();
        step = 1;
    }
    function onUserCanceled() {
        ticket = undefined;
        step = 0;
    }
    function onTicketPaid() {
        step = 2;
    }
    function onTicketFailed() {
        step = 3;
    }
    function onConnectionLost() {
        step = 4;
    }

    let ongoingRaffleInfo: OngoingRaffleInfo;

    $: {
        if (typeof ticket !== "undefined") {
            if (ticket.hasYoutubeVideoData()) {
                mediaKind = "video";
            } else if (ticket.hasSoundcloudTrackData()) {
                mediaKind = "track";
            } else if (ticket.hasDocumentData()) {
                mediaKind = "document";
            }
        }
    }

    onMount(async () => {
        let resp = await apiClient.ongoingRaffleInfo();
        if (resp.hasRaffleInfo()) {
            ongoingRaffleInfo = resp.getRaffleInfo();
        }
    });
</script>

{#if step == 0}
    <EnqueueMediaSelection on:mediaSelected={onMediaSelected} on:userCanceled={() => navigate("/")}>
        <svelte:fragment slot="raffle-info">
            {#if ongoingRaffleInfo !== undefined}
                <EnqueueRafflePromotion {ongoingRaffleInfo} />
            {/if}
        </svelte:fragment>
    </EnqueueMediaSelection>
{:else if step == 1}
    <EnqueuePayment
        on:userCanceled={onUserCanceled}
        on:ticketPaid={onTicketPaid}
        on:ticketExpired={onTicketFailed}
        on:ticketFailed={onTicketFailed}
        on:connectionLost={onConnectionLost}
        bind:ticket
        {mediaKind}
    >
        <svelte:fragment slot="raffle-info">
            {#if ongoingRaffleInfo !== undefined}
                <EnqueueRafflePromotion {ongoingRaffleInfo} />
            {/if}
        </svelte:fragment>
    </EnqueuePayment>
{:else if step == 2}
    <EnqueueSuccess on:enqueueAnother={onUserCanceled} bind:ticket {mediaKind}>
        <svelte:fragment slot="raffle-info">
            {#if ongoingRaffleInfo !== undefined}
                <EnqueueRafflePromotion {ongoingRaffleInfo} />
            {/if}
        </svelte:fragment>
    </EnqueueSuccess>
{:else if step == 3}
    <EnqueueFailure on:enqueueAnother={onUserCanceled} bind:ticket {mediaKind}>
        <svelte:fragment slot="raffle-info">
            {#if ongoingRaffleInfo !== undefined}
                <EnqueueRafflePromotion {ongoingRaffleInfo} />
            {/if}
        </svelte:fragment>
    </EnqueueFailure>
{:else if step == 4}
    <EnqueueFailure on:enqueueAnother={onUserCanceled} bind:ticket connectionLost={true} {mediaKind}>
        <svelte:fragment slot="raffle-info">
            {#if ongoingRaffleInfo !== undefined}
                <EnqueueRafflePromotion {ongoingRaffleInfo} />
            {/if}
        </svelte:fragment>
    </EnqueueFailure>
{/if}
