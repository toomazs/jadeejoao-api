"""One-shot Instagram import for the wedding site.

Reads the couple's PUBLIC profiles with the operator's own account (instagrapi,
unofficial API), downloads each post's display image into the public Supabase
bucket and writes one manifest per person (instagram/{person}.json) that the
Go API serves. The site never talks to Instagram at runtime — run this once,
and again only if the couple ever wants fresher posts.

Usage (from the repo root, inside scripts/.venv):
    python scripts/instagram_import.py
    python scripts/instagram_import.py --code 123456   # when 2FA/challenge asks

Credentials come from the repo's .env: INSTA_USERNAME / INSTA_PASSWORD, plus
SUPABASE_URL / SUPABASE_SECRET_KEY / STORAGE_BUCKET for the uploads. The
logged-in session persists in scripts/.instagram-session.json (gitignored) so
reruns never re-login.
"""

import argparse
import json
import random
import sys
import time
from pathlib import Path
from uuid import uuid4

import requests

REPO_ROOT = Path(__file__).resolve().parents[1]
ENV_PATH = REPO_ROOT / ".env"
SESSION_PATH = Path(__file__).resolve().parent / ".instagram-session.json"

PROFILES = {"bride": "xadenascimento", "groom": "joaodiaspedro"}
POSTS_PER_PROFILE = 12
MEDIA_TYPES = {1: "IMAGE", 2: "VIDEO", 8: "CAROUSEL_ALBUM"}


def load_env(path: Path) -> dict:
    values = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, _, value = line.partition("=")
        values[key.strip()] = value.strip().strip("\"'")
    return values


ENV = load_env(ENV_PATH)
SUPABASE_URL = ENV["SUPABASE_URL"].rstrip("/")
SECRET_KEY = ENV["SUPABASE_SECRET_KEY"]
BUCKET = ENV.get("STORAGE_BUCKET", "jadeejoao-bucket")
USERNAME = ENV["INSTA_USERNAME"]
PASSWORD = ENV["INSTA_PASSWORD"]


def upload(path: str, data: bytes, content_type: str) -> None:
    """Upsert one object into the public bucket via the Storage REST API."""
    response = requests.post(
        f"{SUPABASE_URL}/storage/v1/object/{BUCKET}/{path}",
        headers={
            "Authorization": f"Bearer {SECRET_KEY}",
            "apikey": SECRET_KEY,
            "Content-Type": content_type,
            "x-upsert": "true",
        },
        data=data,
        timeout=60,
    )
    response.raise_for_status()


def public_url(path: str) -> str:
    return f"{SUPABASE_URL}/storage/v1/object/public/{BUCKET}/{path}"


def display_image_url(media) -> str:
    """The best still image for a post: the thumbnail (albums: first slide)."""
    if media.media_type == 8 and media.resources:
        return str(media.resources[0].thumbnail_url or "")
    return str(media.thumbnail_url or "")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--code", help="2FA/challenge verification code, when Instagram asks")
    parser.add_argument(
        "--sessionid",
        help="instagram.com 'sessionid' cookie value — skips password login and 2FA entirely",
    )
    args = parser.parse_args()
    sessionid = args.sessionid or ENV.get("INSTA_SESSIONID", "")

    from instagrapi import Client
    from instagrapi.exceptions import TwoFactorRequired

    def ask_code(where: str) -> str:
        """Interactive terminal: prompt right away. Otherwise: --code flag."""
        if args.code:
            return args.code
        if sys.stdin and sys.stdin.isatty():
            return input(f"Codigo ({where}): ").strip()
        print(f"\nPRECISA DE CODIGO ({where}).")
        print("Pegue o codigo (SMS/WhatsApp/app) e rode de novo:")
        print("  python scripts/instagram_import.py --code XXXXXX")
        sys.exit(2)

    cl = Client()
    cl.delay_range = [1, 3]
    cl.challenge_code_handler = lambda username, choice: ask_code(f"challenge {choice}")

    if SESSION_PATH.exists():
        cl.load_settings(SESSION_PATH)
        print("Sessao anterior carregada.")
    else:
        # Persist the generated device identity BEFORE any login attempt, so
        # every run presents the same "phone" to Instagram instead of a brand
        # new device each time (which is what escalates to checkpoints).
        cl.dump_settings(SESSION_PATH)

    def two_factor_submit(code: str) -> bool:
        """Submit the 2FA code inside the FIRST login attempt's context.

        Calling cl.login() again would fire a new login POST (new SMS, stale
        code); and the library hardcodes verification_method=3 (authenticator
        app), which rejects valid SMS/WhatsApp codes. Here: same identifier,
        detected method, and the device marked trusted so reruns skip 2FA.
        """
        info = (cl.last_json or {}).get("two_factor_info", {})
        identifier = info.get("two_factor_identifier")
        if not identifier:
            raise RuntimeError("contexto 2FA ausente — rode o script de novo")
        if info.get("sms_two_factor_on"):
            method = "1"
        elif info.get("whatsapp_two_factor_on"):
            method = "6"
        else:
            method = "3"
        logged = cl.private_request(
            "accounts/two_factor_login/",
            {
                "verification_code": code,
                "phone_id": cl.phone_id,
                "_csrftoken": cl.token,
                "two_factor_identifier": identifier,
                "username": USERNAME,
                "trust_this_device": "1",
                "guid": cl.uuid,
                "device_id": cl.android_device_id,
                "waterfall_id": str(uuid4()),
                "verification_method": method,
            },
            login=True,
        )
        cl.authorization_data = cl.parse_authorization(
            cl.last_response.headers.get("ig-set-authorization")
        )
        if logged:
            cl.login_flow()
            cl.last_login = time.time()
        return bool(logged)

    try:
        if sessionid:
            print("Entrando com o cookie sessionid do navegador (sem senha, sem 2FA)...")
            if not cl.login_by_sessionid(sessionid):
                print("sessionid invalido ou expirado — pegue um novo no navegador.")
                sys.exit(3)
        else:
            try:
                cl.login(USERNAME, PASSWORD)
            except TwoFactorRequired:
                info = (cl.last_json or {}).get("two_factor_info", {})
                via = (
                    "SMS"
                    if info.get("sms_two_factor_on")
                    else ("WhatsApp" if info.get("whatsapp_two_factor_on") else "app autenticador")
                )
                if not two_factor_submit(ask_code(f"2FA via {via}")):
                    print("Login retornou vazio — rode de novo.")
                    sys.exit(3)
    except SystemExit:
        raise
    except Exception as error:  # noqa: BLE001 — o checkpoint da Meta quebra a lib com erros variados
        print(f"\nLogin falhou: {type(error).__name__}: {error}")
        print("Se apareceu 'checkpoint'/'challenge': abre instagram.com no navegador, resolve")
        print("qualquer aviso ('Foi voce?'), e prefira o caminho do cookie: adicione")
        print("INSTA_SESSIONID=<valor do cookie sessionid> no .env e rode de novo.")
        sys.exit(3)
    cl.dump_settings(SESSION_PATH)
    print("Login ok.")

    for person, handle in PROFILES.items():
        print(f"\nImportando @{handle} ({person})...")
        user_id = cl.user_id_from_username(handle)
        medias = cl.user_medias(user_id, amount=POSTS_PER_PROFILE)
        posts = []
        for media in medias:
            image_url = display_image_url(media)
            if not image_url:
                continue
            image = requests.get(image_url, timeout=60)
            image.raise_for_status()
            key = f"instagram/{person}/{media.pk}.jpg"
            upload(key, image.content, "image/jpeg")
            posts.append(
                {
                    "id": str(media.pk),
                    "caption": (media.caption_text or "")[:300],
                    "media_type": MEDIA_TYPES.get(media.media_type, "IMAGE"),
                    "media_url": public_url(key),
                    "permalink": f"https://www.instagram.com/p/{media.code}/",
                    "timestamp": media.taken_at.isoformat() if media.taken_at else "",
                }
            )
            print(f"  {len(posts):2d}. {posts[-1]['permalink']}")
            time.sleep(random.uniform(1.5, 3.5))

        upload(f"instagram/{person}.json", json.dumps(posts, ensure_ascii=False).encode("utf-8"), "application/json")
        print(f"@{handle}: {len(posts)} posts salvos em instagram/{person}.json")
        time.sleep(random.uniform(3, 6))

    print("\nImportacao concluida. O site ja serve os posts do bucket.")


if __name__ == "__main__":
    main()
