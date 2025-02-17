<script lang="ts">
    import { navigate } from "svelte-navigator";
    import { apiClient } from "./api_client";

    import type { Request } from "@improbable-eng/grpc-web/dist/typings/invoke";
    import { onDestroy } from "svelte";
    import { LabSignInOptions, PermissionLevel, type SignInProgress, type SignInVerification } from "./proto/jungletv_pb";
    import SetRewardsAddressAddressInput from "./SetRewardsAddressAddressInput.svelte";
    import SetRewardsAddressFailure from "./SetRewardsAddressFailure.svelte";
    import SetRewardsAddressSuccess from "./SetRewardsAddressSuccess.svelte";
    import SetRewardsAddressUnopenedAccount from "./SetRewardsAddressUnopenedAccount.svelte";
    import SetRewardsAddressVerification from "./SetRewardsAddressVerification.svelte";
    import { rewardAddress } from "./stores";

    let step = 0;
    let rewardsAddress = "";
    let privilegedLabUserCredential = "";
    let failureReason = "";
    export let verification: SignInVerification;
    function onAddressInput(event: CustomEvent<[string, string]>) {
        rewardsAddress = event.detail[0];
        privilegedLabUserCredential = event.detail[1];
        monitorVerification();
    }
    function onUserCanceled() {
        step = 0;
        if (monitorTicketRequest !== undefined) {
            monitorTicketRequest.close();
        }
    }

    onDestroy(() => {
        if (monitorTicketRequest !== undefined) {
            try {
                monitorTicketRequest.close();
            } catch {}
        }
    });
    let monitorTicketRequest: Request;

    function monitorVerification() {
        monitorTicketRequest = apiClient.signIn(rewardsAddress, handleUpdate, (code, msg) => {
            if (code == 0 || step == 3 || step == 2) {
                return;
            }
            if (code == 2 && msg.includes("Response closed")) {
                setTimeout(monitorVerification, 1000);
                return;
            }
            step = 0;
            if (msg === "invalid reward address") {
                failureReason = "Invalid address for rewards. Make sure this is a valid Banano address.";
            } else if (msg === "rate limit reached") {
                failureReason = "Rate limited due to too many attempts to set an address for rewards.";
            } else {
                failureReason = "Failed to save address due to internal error. Code: " + code + " Message: " + msg;
            }
        }, buildLabSignInOptions());
    }

    function buildLabSignInOptions(): LabSignInOptions {
        if (!globalThis.LAB_BUILD) {
            return undefined;
        }

        const options = new LabSignInOptions();
        options.setDesiredPermissionLevel(PermissionLevel.USER);
        if (privilegedLabUserCredential) {
            options.setDesiredPermissionLevel(PermissionLevel.ADMIN);
            options.setCredential(privilegedLabUserCredential);
        }
        return options;
    }

    function handleUpdate(p: SignInProgress) {
        if (p.hasVerification()) {
            verification = p.getVerification();
            step = 1;
        } else if (p.hasExpired()) {
            step = 3;
            if (monitorTicketRequest !== undefined) {
                monitorTicketRequest.close();
            }
        } else if (p.hasResponse()) {
            apiClient.saveAuthToken(p.getResponse().getAuthToken(), p.getResponse().getTokenExpiration().toDate());
            rewardAddress.update((_) => rewardsAddress);
            step = 2;
            if (monitorTicketRequest !== undefined) {
                monitorTicketRequest.close();
            }
        } else if (p.hasAccountUnopened()) {
            step = 4;
        }
    }
</script>

{#if step == 0}
    <SetRewardsAddressAddressInput
        on:addressInput={onAddressInput}
        on:userCanceled={() => navigate("/")}
        bind:failureReason
    />
{:else if step == 1}
    <SetRewardsAddressVerification on:userCanceled={onUserCanceled} bind:verification />
{:else if step == 2}
    <SetRewardsAddressSuccess bind:rewardsAddress />
{:else if step == 3}
    <SetRewardsAddressFailure on:tryAgain={onUserCanceled} />
{:else if step == 4}
    <SetRewardsAddressUnopenedAccount on:userCanceled={onUserCanceled} />
{/if}
