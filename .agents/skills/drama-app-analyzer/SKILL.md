---

## name: drama-app-analyzer

description: Analytical profile of the ShortsWave (novel/short drama) mobile application. Breaks down functional modules and core interaction logic for AI context.  
version: 1.0.0

# ShortsWave App Functional Profile

> [!NOTE]  
> **Integration**: This skill serves as a domain-specific knowledge plugin for the [testcase-generator](file:///Users/apple/TestCenter_Vue3/.agents/skills/testcase-generator/SKILL.md). When generating test cases for ShortsWave, use the logical constraints defined here to satisfy the 4-dimension (Positive/Negative/Exception/Concurrency) requirements of the generator.

This skill provides a comprehensive breakdown of the ShortsWave application, a content platform for short dramas and novel-like serialized entertainment.

## 1. Functional Modules List

### 🎞️ Home (首页)

- **Header Components**:
  - **Search Bar**: Top-aligned search entry, leads to Search Page.
  - **VIP Icon**: Located next to the search bar, redirects to **VIP Center**.
  - **Task Icon**: Right-most icon in the header, jumps to the **Rewards Page**.
- **Module/Slot Types (版位种类)**:
  - **Banners**: Large top-rotating featured content (Carousel).
  - **Auto-play Slots**: Content automatically plays when the cover stops in the middle of the screen.
  - **VIP Premium Slots**: Unique dramas restricted to VIP subscribers.
  - **Quantity Layouts**: Grouped content (sets of 3 or 6) with varying sizes and typography based on the group size.
  - **Reservation Slots**: Only the first title is watchable; others are locked but allow "Reservation" clicks to trigger push notifications.
  - **Activity Icons**: Floating or fixed pointers to current marketing events/promotions.
  - **Waterfall Feed**: A comprehensive collection of all available dramas.
    - **Internal Rankings**: Categorized "Leaderboards" placed within the scroll for focused discovery (clickable to dedicated category pages).

### 🎬 For You

- **Tabbed Navigation**: Toggle between **For You** and **More** tabs.
- **For You (Preview Interface)**:
  - **Main Player**: Central video area supporting **Glossary (术语表)** and **Subtitles (字幕)** overlay.
  - **Metadata Overlay (Bottom-Left)**: Displays Drama Cover, Title, **Heat Tags (热力标签)**, and **Category Tags (分类标签)**.
  - **Episode Info**: Displays total episode count.
  - **Full Episodes Button**: Large CTA to jump to the official full-screen player page for the current drama.
  - **Side Actions (Right)**: Vertical action bar with **Favorite (收藏)** and **Share (转发)** buttons.
- **More (Library/Resource Center)**:
  - **Category Filters**: Dedicated resource library with attribute filtering.
  - **Supported Genres**: Male/Female oriented (男频/女频), Modern, Romance, Strike Back (逆袭), etc.

### 📖 Content Player / Reader (内容播放/阅读)

- **Vertical Full-screen Player**: Immersive video playback for 1-2 minute episodes.
- **Episode Manager**: Chapter list selector, auto-play toggle, and speed control.
- **Social Engagement**: Double-tap to like, comment threads, and deep-link sharing.

### 🎁 Rewards (任务页)

- **Main Task Group (主页任务组)**:
  - **Incentivized Tasks (激励任务)**: Standard engagement rewards.
  - **Starmobi Tasks**: Special third-party task integration.
- **Fixed Task Banner (固定任务Banner)**: Top-fixed carousel featuring 5 distinct incentivized tasks.
- **Video Task Group (视频任务组)**: Progression-based rewards triggered by collective/individual video watching duration.
- **Ad Task Group (广告任务组)**: Rewards specifically linked to ad viewing actions.

### 📚 My List (剧架)

- **My Like Tab**: 
  - **Favorites**: List of dramas manually favorited by the user.
  - **Interaction**: Users can **Un-favorite (取消收藏)** content directly from this list.
  - **Recommendations**: Algorithmically generated suggestions based on likes.
- **History Tab**: 
  - **Watch History**: Record of previously viewed dramas and episodes.
  - **Interaction**: Supports **Edit/Batch Delete (编辑/删除)** to clear specific or all records.
  - **Recommendations**: Algorithmically generated suggestions based on viewing habits.

### 💰 Wallet & Monetization (钱包/付费)

- **IAP Top-up**: Tiered coin packages for purchasing premium episodes.
- **Subscription Models**: VIP passes (Monthly/Weekly) for ad-free experience and full access.
- **Consumption History**: Detailed logs of coin usage and unlocks.

### 👤 Me (个人中心)

- **User Identity Header**:
  - **Profiles**: Displays Thumbnail, Nickname, and User ID (UID).
  - **Sign In**: Toggle button for account binding; anonymous/guest info shown by default, synced account info shown after login.
- **VIP Center**: Dedicated entry for membership status and privilege management.
- **My Wallet**:
  - **Balances**: Displays real-time **Coins** and **Bonus** amounts.
  - **Top Up**: Leads to the store for **IAP (one-time coin packs)** and **Subscription (VIP recurring access)**.
- **Continue Watching**: 
  - **Quick Access**: List of recently viewed dramas/episodes (synced with History).
  - **More**: Button to jump to the full **History** tab in My List.
- **Rate Us**: Application feedback system using a 5-star rating scale.
- **Download**: Offline video caching feature (restricted to **VIP Subscribers**).
- **My List Access**: Direct navigation link to the **My List (剧架)** page.
- **Feedback**: Triggers the native email client for specialized reporting.
- **Settings (高级设置)**:
  - **About**: Includes Privacy Policy, Terms of Service, and **Account Deletion** trigger.
  - **Auto-unlock**: Toggle for automatic coin deduction for subsequent episodes.
  - **Auto-play**: Toggle for whether the app should transition to the Shorts/ForYou page and start playback upon launch.
  - **Language**: Localization switcher.

## 2. Core Interaction Logic

1. **Infinite Swiping (沉浸式交互)**: UI follows a vertical swipe pattern (TikTok-style) for rapid content consumption and low-friction discovery.
2. **Micro-transaction Friction (微付费漏斗)**: Provides 5-10 free episodes to hook users, then triggers a payment/ad-gate for continuing.
3. **Engagement Loop (激励闭环)**: Circular flow between "Earn Coins (Tasks) -> Consume Content -> Spend Coins -> Earn More Coins".
4. **Offline Persistence (离线缓存)**: Support for downloading chapters/episodes for bandwidth-limited regions.

## 3. Key UI Elements for Automation

- `home_tab_discovery`: Main feed entry.
- `home_tab_foryou`: Filter tab for previews.
- `home_tab_more`: Filter tab for library.
- `home_search_bar`: Entry to search page.
- `home_vip_icon`: Entry to VIP center.
- `home_task_icon`: Entry to reward/task page.
- `home_reserve_btn`: Reservation button in pre-order slots.
- `foryou_full_episodes_btn`: Jump to full player.
- `foryou_favorite_icon`: Toggle favorite.
- `foryou_share_icon`: Open share dialog.
- `more_filter_gender`: Gender filter options.
- `more_filter_genre`: Genre (Romance, etc.) filter.
- `player_next_episode`: Swipe container or button.
- `task_check_in_btn`: Reward trigger.
- `payment_iap_tier_1`: Purchase button.
- `profile_history_list`: Progress retrieval.