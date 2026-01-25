import { useState } from "react";
import { useApp } from "../context/AppContext";
import {
  searchPodcasts,
  fetchPodcastEpisodes,
  postDownload,
  type PodcastChannel,
  type PodcastEpisode,
  type PodcastSearchResult,
} from "../utils/apis";
import { FaArrowLeft, FaPlay, FaPodcast } from "react-icons/fa6";

type ViewState =
  | { type: "search" }
  | { type: "results"; results: PodcastSearchResult[] }
  | { type: "channel"; channel: PodcastChannel; episodes: PodcastEpisode[] };

export function PodcastPage() {
  const { isConnected, t, configLang, showToast } = useApp();

  const [query, setQuery] = useState("");
  const [searching, setSearching] = useState(false);
  const [loading, setLoading] = useState(false);
  const [viewState, setViewState] = useState<ViewState>({ type: "search" });

  const handleSearch = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!query.trim() || searching) return;

    setSearching(true);
    try {
      const res = await searchPodcasts(query.trim(), configLang);
      if (res.code === 200) {
        setViewState({ type: "results", results: res.data.results });
      }
    } finally {
      setSearching(false);
    }
  };

  const handleChannelClick = async (channel: PodcastChannel) => {
    setLoading(true);
    try {
      const res = await fetchPodcastEpisodes(channel.id, channel.source);
      if (res.code === 200) {
        setViewState({
          type: "channel",
          channel,
          episodes: res.data.episodes,
        });
      }
    } finally {
      setLoading(false);
    }
  };

  const handleEpisodeClick = async (episode: PodcastEpisode) => {
    // Download the episode by submitting its URL
    if (episode.download_url) {
      // Use podcast name + episode title as filename
      const filename = `${episode.podcast_name} - ${episode.title}`;
      const res = await postDownload(episode.download_url, filename);
      if (res.code === 200) {
        showToast("success", t.podcast_download_started);
      }
    }
  };

  const handleBack = () => {
    if (viewState.type === "channel") {
      // Go back to results if we have them
      setViewState({ type: "search" });
      // Re-trigger search with existing query
      if (query.trim()) {
        handleSearchWithQuery(query.trim());
      }
    }
  };

  const handleSearchWithQuery = async (q: string) => {
    setSearching(true);
    try {
      const res = await searchPodcasts(q, configLang);
      if (res.code === 200) {
        setViewState({ type: "results", results: res.data.results });
      }
    } finally {
      setSearching(false);
    }
  };

  const formatDuration = (seconds: number): string => {
    if (seconds <= 0) return "?";
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = seconds % 60;
    if (h > 0) {
      return `${h}:${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
    }
    return `${m}:${s.toString().padStart(2, "0")}`;
  };

  const formatDate = (dateStr?: string): string => {
    if (!dateStr) return "";
    try {
      const date = new Date(dateStr);
      return date.toLocaleDateString();
    } catch {
      return "";
    }
  };

  // Get all channels and episodes from results
  const getAllChannels = (): PodcastChannel[] => {
    if (viewState.type !== "results") return [];
    return viewState.results.flatMap((r) => r.podcasts);
  };

  const getAllEpisodes = (): PodcastEpisode[] => {
    if (viewState.type !== "results") return [];
    return viewState.results.flatMap((r) => r.episodes);
  };

  return (
    <div className="max-w-3xl mx-auto flex flex-col gap-4">
      {/* Search Bar */}
      <form className="flex flex-col sm:flex-row gap-3" onSubmit={handleSearch}>
        <div className="flex gap-3 flex-1">
          {viewState.type === "channel" && (
            <button
              type="button"
              onClick={handleBack}
              className="px-4 py-3 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-900 text-zinc-700 dark:text-zinc-300 hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors shrink-0"
            >
              <FaArrowLeft />
            </button>
          )}
          <input
            type="text"
            className="flex-1 min-w-0 px-4 py-3 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-900 text-zinc-900 dark:text-white text-base focus:outline-none focus:border-blue-500 placeholder:text-zinc-400 dark:placeholder:text-zinc-600 disabled:opacity-50"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t.podcast_search_hint}
            disabled={!isConnected || searching}
          />
        </div>
        <button
          type="submit"
          className="px-6 py-3 border-none rounded-lg bg-blue-500 text-white text-base font-medium cursor-pointer hover:bg-blue-600 disabled:bg-zinc-300 dark:disabled:bg-zinc-700 disabled:cursor-not-allowed transition-colors"
          disabled={!isConnected || !query.trim() || searching}
        >
          {searching ? t.podcast_searching : t.podcast_search}
        </button>
      </form>

      {/* Loading State */}
      {(searching || loading) && (
        <div className="text-center py-12 text-zinc-400 dark:text-zinc-600">
          {t.loading}
        </div>
      )}

      {/* Search Results View */}
      {!searching && !loading && viewState.type === "results" && (
        <div className="flex flex-col gap-6">
          {getAllChannels().length === 0 && getAllEpisodes().length === 0 ? (
            <div className="text-center py-12 text-zinc-400 dark:text-zinc-600">
              <p>{t.podcast_no_results}</p>
            </div>
          ) : (
            <>
              {/* Channels Section */}
              {getAllChannels().length > 0 && (
                <section>
                  <h2 className="text-sm font-medium text-zinc-700 dark:text-zinc-200 mb-3">
                    {t.podcast_channels} ({getAllChannels().length})
                  </h2>
                  <div className="flex flex-col gap-2">
                    {getAllChannels().map((channel) => (
                      <div
                        key={`${channel.source}-${channel.id}`}
                        onClick={() => handleChannelClick(channel)}
                        className="flex items-center gap-3 p-3 border border-zinc-200 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-900 hover:bg-zinc-50 dark:hover:bg-zinc-800 cursor-pointer transition-colors"
                      >
                        <div className="w-10 h-10 rounded-lg bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center text-blue-500">
                          <FaPodcast />
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="font-medium text-zinc-900 dark:text-white truncate">
                            {channel.title}
                          </div>
                          <div className="text-sm text-zinc-500 dark:text-zinc-400 truncate">
                            {channel.author} | {channel.episode_count}{" "}
                            {t.podcast_episodes_count}
                          </div>
                        </div>
                        <div className="text-xs text-zinc-400 dark:text-zinc-500 uppercase">
                          {channel.source}
                        </div>
                      </div>
                    ))}
                  </div>
                </section>
              )}

              {/* Episodes Section */}
              {getAllEpisodes().length > 0 && (
                <section>
                  <h2 className="text-sm font-medium text-zinc-700 dark:text-zinc-200 mb-3">
                    {t.podcast_episodes} ({getAllEpisodes().length})
                  </h2>
                  <div className="flex flex-col gap-2">
                    {getAllEpisodes().map((episode) => (
                      <div
                        key={`${episode.source}-${episode.id}`}
                        onClick={() => handleEpisodeClick(episode)}
                        className="flex items-center gap-3 p-3 border border-zinc-200 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-900 hover:bg-zinc-50 dark:hover:bg-zinc-800 cursor-pointer transition-colors"
                      >
                        <div className="w-10 h-10 rounded-lg bg-green-100 dark:bg-green-900/30 flex items-center justify-center text-green-500">
                          <FaPlay className="text-sm" />
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="font-medium text-zinc-900 dark:text-white truncate">
                            {episode.title}
                          </div>
                          <div className="text-sm text-zinc-500 dark:text-zinc-400 truncate">
                            {episode.podcast_name} | {formatDuration(episode.duration)}
                            {episode.pub_date && ` | ${formatDate(episode.pub_date)}`}
                          </div>
                        </div>
                        <div className="text-xs text-zinc-400 dark:text-zinc-500 uppercase">
                          {episode.source}
                        </div>
                      </div>
                    ))}
                  </div>
                </section>
              )}
            </>
          )}
        </div>
      )}

      {/* Channel Episodes View */}
      {!loading && viewState.type === "channel" && (
        <div className="flex flex-col gap-4">
          {/* Channel Header */}
          <div className="flex items-center gap-3 p-4 border border-zinc-200 dark:border-zinc-700 rounded-lg bg-zinc-50 dark:bg-zinc-800">
            <div className="w-12 h-12 rounded-lg bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center text-blue-500 text-xl">
              <FaPodcast />
            </div>
            <div className="flex-1 min-w-0">
              <div className="font-medium text-zinc-900 dark:text-white text-lg">
                {viewState.channel.title}
              </div>
              <div className="text-sm text-zinc-500 dark:text-zinc-400">
                {viewState.channel.author}
              </div>
            </div>
          </div>

          {/* Episodes List */}
          <section>
            <h2 className="text-sm font-medium text-zinc-700 dark:text-zinc-200 mb-3">
              {t.podcast_episodes} ({viewState.episodes.length})
            </h2>
            {viewState.episodes.length === 0 ? (
              <div className="text-center py-8 text-zinc-400 dark:text-zinc-600">
                {t.podcast_no_results}
              </div>
            ) : (
              <div className="flex flex-col gap-2">
                {viewState.episodes.map((episode) => (
                  <div
                    key={`${episode.source}-${episode.id}`}
                    onClick={() => handleEpisodeClick(episode)}
                    className="flex items-center gap-3 p-3 border border-zinc-200 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-900 hover:bg-zinc-50 dark:hover:bg-zinc-800 cursor-pointer transition-colors"
                  >
                    <div className="w-10 h-10 rounded-lg bg-green-100 dark:bg-green-900/30 flex items-center justify-center text-green-500">
                      <FaPlay className="text-sm" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="font-medium text-zinc-900 dark:text-white truncate">
                        {episode.title}
                      </div>
                      <div className="text-sm text-zinc-500 dark:text-zinc-400">
                        {formatDuration(episode.duration)}
                        {episode.pub_date && ` | ${formatDate(episode.pub_date)}`}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </section>
        </div>
      )}

      {/* Initial State */}
      {!searching && !loading && viewState.type === "search" && (
        <div className="text-center py-12 text-zinc-400 dark:text-zinc-600">
          <p>{t.podcast_search_hint}</p>
        </div>
      )}
    </div>
  );
}
