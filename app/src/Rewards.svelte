<script lang="ts">
    import { Moon } from "svelte-loading-spinners";
    import { link, navigate } from "svelte-navigator";
    import AccountConnections from "./AccountConnections.svelte";
    import { apiClient } from "./api_client";
    import { openUserProfile } from "./profile_utils";
    import type { PaginationParameters } from "./proto/common_pb";
    import type { Connection, PointsInfoResponse, ReceivedReward, ServiceInfo, Withdrawal } from "./proto/jungletv_pb";
    import ErrorMessage from "./uielements/ErrorMessage.svelte";
    import PaginatedTable from "./uielements/PaginatedTable.svelte";

    import { formatBANPrice } from "./currency_utils";
    import { modalConfirm } from "./modal/modal";
    import { badRepresentative, currentSubscription, darkMode, rewardAddress, rewardBalance } from "./stores";
    import ReceivedRewardTableItem from "./tableitems/ReceivedRewardTableItem.svelte";
    import WithdrawalTableItem from "./tableitems/WithdrawalTableItem.svelte";
    import ButtonButton from "./uielements/ButtonButton.svelte";
    import SuccessMessage from "./uielements/SuccessMessage.svelte";
    import WarningMessage from "./uielements/WarningMessage.svelte";
    import Wizard from "./uielements/Wizard.svelte";
    import { hrefButtonStyleClasses, isSubscriptionAboutToExpire } from "./utils";

    let pendingWithdrawal = false;
    let withdrawalPositionInQueue = 0;
    let withdrawalsInQueue = 0;
    let connections: Connection[];
    let services: ServiceInfo[];

    let rewardInfoPromise = (async function () {
        try {
            let rewardInfo = await apiClient.rewardInfo();

            rewardAddress.update((_) => rewardInfo.getRewardsAddress());
            rewardBalance.update((_) => rewardInfo.getRewardBalance());
            badRepresentative.update((_) => rewardInfo.getBadRepresentative());
            pendingWithdrawal = rewardInfo.getWithdrawalPending();
            if (rewardInfo.hasWithdrawalPositionInQueue()) {
                withdrawalPositionInQueue = rewardInfo.getWithdrawalPositionInQueue();
            }
            if (rewardInfo.hasWithdrawalsInQueue()) {
                withdrawalsInQueue = rewardInfo.getWithdrawalsInQueue();
            }
        } catch (ex) {
            console.log(ex);
            navigate("/rewards/address");
        }
    })();

    async function loadConnections() {
        try {
            let r = await apiClient.connections();
            connections = r.getConnectionsList();
            services = r.getServiceInfosList();
        } catch (ex) {
            console.log(ex);
            navigate("/rewards/address");
        }
    }

    let connectionsPromise = loadConnections();

    let withdrawClicked = false;
    let withdrawSuccessful = false;
    let withdrawFailed = false;
    async function withdraw() {
        if (withdrawClicked) {
            return;
        }
        withdrawFailed = false;
        withdrawSuccessful = false;
        withdrawClicked = true;
        try {
            await apiClient.withdraw();
            withdrawSuccessful = true;
            rewardBalance.update((_) => "0");
        } catch (e) {
            withdrawFailed = true;
            console.log(e);
        }
        withdrawClicked = false;
    }

    let cur_received_rewards_page = 0;
    async function getReceivedRewardsPage(pagParams: PaginationParameters): Promise<[ReceivedReward[], number]> {
        let resp = await apiClient.rewardHistory(pagParams);
        return [resp.getReceivedRewardsList(), resp.getTotal()];
    }

    let cur_withdrawals_page = 0;
    async function getWithdrawalsPage(pagParams: PaginationParameters): Promise<[Withdrawal[], number]> {
        let resp = await apiClient.withdrawalHistory(pagParams);
        return [resp.getWithdrawalsList(), resp.getTotal()];
    }

    async function pointsPromise(): Promise<PointsInfoResponse> {
        let response = await apiClient.pointsInfo();
        $currentSubscription = response.getCurrentSubscription();
        return response;
    }

    $: currentSubAboutToExpire = isSubscriptionAboutToExpire($currentSubscription);

    async function signOut() {
        if (
            await modalConfirm(
                "You will stop receiving rewards for watching and you will no longer be able to participate in chat, until you authenticate with your Banano address again. Continue?",
                "Sign out?",
                "Sign out",
                "Cancel"
            )
        ) {
            apiClient.signOut();
            rewardAddress.update((_) => "");
            navigate("/rewards/address");
        }
    }

    async function invalidateAuthTokens() {
        if (
            await modalConfirm(
                "This will sign you out on every browser where you've registered to receive rewards with this Banano address, including the one you are currently using.\n" +
                    "You will stop receiving rewards for watching and you will no longer be able to participate in chat, until you reauthenticate on all the devices where you wish to do so.\n\n" +
                    "You should use this option if you forgot to sign out of JungleTV on a shared device, or if you believe you've been a victim of cookie stealing/session hijacking.",
                "Sign out everywhere?",
                "Sign out everywhere",
                "Cancel"
            )
        ) {
            apiClient.invalidateAuthTokens();
            apiClient.signOut();
            rewardAddress.update((_) => "");
            navigate("/rewards/address");
        }
    }
</script>

<Wizard>
    <div slot="step-info">
        <h3 class="text-lg font-semibold leading-6 text-gray-900 dark:text-gray-200">Receive rewards</h3>
        <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">
            When a queue entry finishes playing, the amount it cost to enqueue is distributed evenly among eligible
            users. To minimize the number of Banano transactions caused by JungleTV, rewards are added to a balance
            before they are sent to you. You can wait for an automated withdrawal or withdraw manually at any time.
        </p>
        <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">
            Some content has e.g. regional restrictions and may not display for you. You will still be rewarded as long
            as you keep the JungleTV page open throughout the duration of the queue entry.
        </p>
        <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">
            If you have watched multiple pieces of content and have not received a reward, please confirm that you are
            not using a VPN or proxy and that you did not violate the <a use:link href="/guidelines">Guidelines</a>.
        </p>
    </div>
    <div slot="main-content">
        {#if globalThis.LAB_BUILD}
            <WarningMessage>
                This is a lab environment where users cannot withdraw rewards. The rewards system otherwise works as in
                the production version of the website, but users will never be able to withdraw their balance. Banano
                received goes towards the upkeeping of this lab environment.
                <br />
                <strong>Please ignore any UI text mentioning the ability to receive rewards.</strong>
            </WarningMessage>
        {/if}
        {#await rewardInfoPromise}
            <p><Moon size="28" color={$darkMode ? "#FFFFFF" : "#444444"} unit="px" duration="2s" /></p>
        {:then}
            <p class="text-lg font-semibold">Currently rewarding:</p>
            <p class="font-mono text-sm break-words">{$rewardAddress}</p>
            <div class="mt-2 mb-6 flex flex-row gap-4 sm:gap-6">
                <a use:link href="/rewards/address" class={hrefButtonStyleClasses()}>Change address</a>
                <ButtonButton on:click={() => openUserProfile($rewardAddress)}>Edit profile and nickname</ButtonButton>
            </div>
            {#if pendingWithdrawal}
                <div class="mt-3">
                    <WarningMessage>
                        A withdrawal is pending for your account. This usually takes some seconds to complete, and
                        occasionally can take some minutes. You'll be able to withdraw when it completes.
                        <br />
                        Your withdrawal is in position {withdrawalPositionInQueue} of {withdrawalsInQueue} withdrawals in
                        queue to be processed.
                    </WarningMessage>
                </div>
                <p class="text-lg font-semibold">Current balance:</p>
            {:else}
                <p class="text-lg font-semibold">Available to withdraw:</p>
            {/if}
            <p class="text-2xl sm:text-3xl">
                {formatBANPrice($rewardBalance)} <span class="text-xl">BAN</span>
            </p>
            {#if !pendingWithdrawal}
                <p class="mt-2 mb-6">
                    {#if withdrawSuccessful}
                        <SuccessMessage>
                            Withdraw request successful. You'll receive Banano in your account soon.
                        </SuccessMessage>
                    {:else if withdrawFailed}
                        <ErrorMessage>
                            Withdraw request failed. It is possible that a withdraw request is already in progress.
                            Please try again later.
                        </ErrorMessage>
                    {:else if parseFloat(formatBANPrice($rewardBalance)) > 0}
                        <ButtonButton
                            on:click={withdraw}
                            extraClasses={withdrawClicked ? "animate-pulse" : ""}
                            color={withdrawClicked ? "gray" : "yellow"}
                        >
                            Withdraw
                        </ButtonButton>
                    {/if}
                </p>
            {/if}
            <p class="mt-4">
                Withdrawals happen automatically when your balance reaches 10 BAN, or 24 hours after your last received
                reward, whichever happens first.
            </p>
        {/await}
    </div>
    <div slot="extra_1">
        <div class="shadow sm:rounded-md sm:overflow-hidden">
            <div class="px-4 py-5 bg-white dark:bg-gray-800 space-y-6 sm:p-6">
                <div class="flex flex-row gap-4 sm:gap-6 items-center">
                    <img src="/assets/brand/points.svg" alt="JungleTV Points" title="JungleTV Points" class="h-16" />
                    <div class="grow">
                        <p class="text-lg font-semibold text-gray-800 dark:text-white">JungleTV Points</p>
                        <p class="text-sm">Participate in the community and earn points to spend in JungleTV.</p>
                        {#if typeof $currentSubscription !== "undefined" && $currentSubscription != null}
                            <p class="text-sm">
                                Your <a
                                    href="/points#nice"
                                    use:link
                                    class="font-semibold text-green-500 dark:text-green-300"
                                >
                                    JungleTV Nice
                                </a>
                                membership gets you awesome perks{#if currentSubAboutToExpire}
                                    <span class="font-semibold text-red-600 dark:text-red-400">
                                        &nbsp;and is about to expire</span
                                    >{/if}.
                            </p>
                        {:else}
                            <p class="text-sm">
                                Upgrade to <a
                                    href="/points#nice"
                                    use:link
                                    class="font-semibold text-green-500 dark:text-green-300"
                                >
                                    JungleTV Nice
                                </a> to get awesome perks!
                            </p>
                        {/if}
                    </div>
                </div>
                <div class="flex flex-col sm:flex-row gap-4 sm:gap-6">
                    <div class="grow">
                        You have
                        {#await pointsPromise()}
                            <span class="inline-block">
                                <Moon size="28" color={$darkMode ? "#FFFFFF" : "#444444"} unit="px" duration="2s" />
                            </span>
                        {:then response}
                            <span class="text-2xl sm:text-3xl">{response.getBalance()}</span>
                        {/await}
                        points.
                    </div>
                    <div class="flex flex-row gap-4 sm:gap-6">
                        <a use:link href="/points/frombanano" class={hrefButtonStyleClasses()}>
                            Get points with Banano
                        </a>
                        <a use:link href="/points" class={hrefButtonStyleClasses("green")}>Learn more</a>
                    </div>
                </div>
            </div>
        </div>
    </div>
    <div slot="extra_2">
        <div class="shadow sm:rounded-md sm:overflow-hidden">
            <div class="px-4 py-5 bg-white dark:bg-gray-800 space-y-6 sm:p-6">
                <div class="grid grid-cols-3 gap-6">
                    <div class="col-span-3">
                        {#await connectionsPromise}
                            <p><Moon size="28" color={$darkMode ? "#FFFFFF" : "#444444"} unit="px" duration="2s" /></p>
                        {:then}
                            <AccountConnections {connections} {services} on:needsUpdate={loadConnections} />
                        {/await}
                    </div>
                </div>
            </div>
        </div>
    </div>
    <div slot="extra_3">
        <PaginatedTable
            title={"Received rewards"}
            column_count={3}
            error_message={"Error loading received rewards"}
            no_items_message={"No rewards received yet"}
            data_promise_factory={getReceivedRewardsPage}
            bind:cur_page={cur_received_rewards_page}
        >
            <tr slot="thead">
                <th
                    class="px-4 sm:px-6 align-middle border border-solid py-3 text-xs uppercase
                        border-l-0 border-r-0 whitespace-nowrap font-semibold text-left
                bg-gray-100 text-gray-600 border-gray-200 dark:bg-gray-700 dark:text-gray-400 dark:border-gray-600"
                >
                    Amount
                </th>
                <th
                    class="px-4 sm:px-6 align-middle border border-solid py-3 text-xs uppercase
                        border-l-0 border-r-0 whitespace-nowrap font-semibold text-left
            bg-gray-100 text-gray-600 border-gray-200 dark:bg-gray-700 dark:text-gray-400 dark:border-gray-600"
                >
                    Received at
                </th>
                <th
                    class="px-4 sm:px-6 align-middle border border-solid py-3 text-xs uppercase
                        border-l-0 border-r-0 whitespace-nowrap font-semibold text-left
                        bg-gray-100 text-gray-600 border-gray-200 dark:bg-gray-700 dark:text-gray-400 dark:border-gray-600"
                >
                    Media
                </th>
            </tr>

            <tbody slot="item" let:item class="hover:bg-gray-200 dark:hover:bg-gray-700">
                <ReceivedRewardTableItem reward={item} />
            </tbody>
        </PaginatedTable>
    </div>
    <div slot="extra_4">
        <PaginatedTable
            title={"Completed withdrawals"}
            column_count={4}
            error_message={"Error loading completed withdrawals"}
            no_items_message={"No withdrawals"}
            data_promise_factory={getWithdrawalsPage}
            bind:cur_page={cur_withdrawals_page}
        >
            <tr slot="thead">
                <th
                    class="px-4 sm:px-6 align-middle border border-solid py-3 text-xs uppercase
                        border-l-0 border-r-0 whitespace-nowrap font-semibold text-left
                bg-gray-100 text-gray-600 border-gray-200 dark:bg-gray-700 dark:text-gray-400 dark:border-gray-600"
                >
                    Amount
                </th>
                <th
                    class="px-4 sm:px-6 align-middle border border-solid py-3 text-xs uppercase
                        border-l-0 border-r-0 whitespace-nowrap font-semibold text-left
            bg-gray-100 text-gray-600 border-gray-200 dark:bg-gray-700 dark:text-gray-400 dark:border-gray-600"
                >
                    Initiated at
                </th>
                <th
                    class="px-4 sm:px-6 align-middle border border-solid py-3 text-xs uppercase
                        border-l-0 border-r-0 whitespace-nowrap font-semibold text-left
                        bg-gray-100 text-gray-600 border-gray-200 dark:bg-gray-700 dark:text-gray-400 dark:border-gray-600"
                >
                    Completed
                </th>
                <th
                    class="px-4 sm:px-6 align-middle border border-solid py-3 text-xs uppercase
                        border-l-0 border-r-0 whitespace-nowrap font-semibold text-left
                        bg-gray-100 text-gray-600 border-gray-200 dark:bg-gray-700 dark:text-gray-400 dark:border-gray-600"
                />
            </tr>

            <tbody slot="item" let:item class="hover:bg-gray-200 dark:hover:bg-gray-700">
                <WithdrawalTableItem withdrawal={item} />
            </tbody>
        </PaginatedTable>
    </div>
    <div slot="extra_5">
        <div class="shadow sm:rounded-md sm:overflow-hidden">
            <div class="px-4 py-5 bg-white dark:bg-gray-800 space-y-6 sm:p-6">
                <div class="grid grid-cols-3 gap-6">
                    <div class="col-span-3">
                        <p class="text-lg font-semibold text-gray-800 dark:text-white">Account security</p>
                        <div class="mt-2 mb-6 flex flex-row gap-4 sm:gap-6">
                            <ButtonButton on:click={signOut}>Sign out</ButtonButton>
                            <ButtonButton color="red" on:click={invalidateAuthTokens}>Sign out everywhere</ButtonButton>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</Wizard>
