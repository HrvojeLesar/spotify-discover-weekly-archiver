import spotipy
import time
from spotipy.oauth2 import SpotifyOAuth
from dotenv import load_dotenv
from datetime import datetime
import schedule

DISCOVER_WEEKLY_PLAYLIST_NAME = "Discover Weekly"
DISCOVER_WEEKLY_PLAYLIST_OWNER_ID = "spotify"

class DiscoverWeeklyArchiver():
    def __init__(self) -> None:
        scopes = [
                "playlist-read-private",
                "playlist-modify-public",
                "playlist-modify-private"
                ]
        self.sp = spotipy.Spotify(auth_manager=SpotifyOAuth(scope=scopes, open_browser=False))
        self.find_discover_weekly_playlist()
        self.get_discover_weekly_tracks()
        if self.is_discover_weekly_archived() == False:
            self.archive()
            print("Successfully archived playlist!")
        else:
            print("Playlist already archived")

    def find_discover_weekly_playlist(self):
        playlists = self.sp.current_user_playlists()
        if len(playlists["items"]) == 0:
            raise Exception("User has no playlists")
        while True:
            for playlist in playlists["items"]:
                if playlist["name"] == DISCOVER_WEEKLY_PLAYLIST_NAME and playlist["owner"]["id"] == DISCOVER_WEEKLY_PLAYLIST_OWNER_ID:
                    self.dw = playlist
                    return
                    
            if playlists["next"]:
                playlists = self.sp.next(playlists)
            else:
                break
        
        raise Exception("Discover weekly not found")

    def get_discover_weekly_tracks(self):
        self.tracks = self.sp.playlist_tracks(self.dw["id"])
        self.sorted_tracks = sorted(self.tracks["items"], key=lambda track: track["track"]["id"])

    def is_discover_weekly_archived(self):
        playlists = self.sp.current_user_playlists()
        if len(playlists["items"]) == 0:
            raise Exception("User has no playlists")
        while True:
            for playlist in playlists["items"]:
                if playlist["tracks"]["total"] != 30 or (playlist["name"] == DISCOVER_WEEKLY_PLAYLIST_NAME and playlist["owner"]["id"] == DISCOVER_WEEKLY_PLAYLIST_OWNER_ID):
                    continue

                tracks = self.sp.playlist_tracks(playlist["id"])
                tracks["items"].sort(key=lambda track: track["track"]["id"])
                tracks = tracks["items"]

                for i in range(0, 30):
                    if tracks[i]["track"]["id"] != self.sorted_tracks[i]["track"]["id"]:
                        break
                    if i == 29:
                        return True

            if playlists["next"]:
                playlists = self.sp.next(playlists)
            else:
                break
        
        return False

    def archive(self):
        dt = datetime.now()
        playlist_name = dt.strftime("%d-%m-%y DW")
        user_id = self.sp.current_user()["id"]
        created_playlist = self.sp.user_playlist_create(user_id, playlist_name, public=True)

        track_ids = [track["track"]["id"] for track in self.tracks["items"]]
        self.sp.user_playlist_add_tracks(user_id, created_playlist["id"], track_ids)
        
def archive():
    print("Running archive job...")
    DiscoverWeeklyArchiver()
    print("Finished running job")

def main():
    load_dotenv()
    schedule.every().monday.at("03:00").do(archive)

    schedule.run_all()

    while True:
        schedule.run_pending()
        time.sleep(1)

if __name__ == "__main__":
    main()
