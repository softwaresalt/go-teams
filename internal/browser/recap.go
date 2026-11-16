package browser

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

	"github.com/softwaresalt/go-teams/internal/audio"
)

// RecapOffsetSeparator separates the speaker display name from an HH:MM:SS
// recording offset when delivering Meeting Recap transcript entries to a
// CaptionSink.
const RecapOffsetSeparator = "\x00"

const extractRecapAudioJS = `(async () => {
	const limitError = 'recap audio exceeds maximum supported size/duration';
	try { delete window.__goTeamsRecapAudioTransfer; } catch (_) {}
	const enforceIdentity = typeof recapExpectedIdentity !== 'undefined';
	const expected = enforceIdentity ? recapExpectedIdentity : {};
	const authHosts = [
		'login.microsoftonline.com',
		'login.microsoft.com',
		'login.live.com',
		'account.live.com',
		'login.windows.net',
		'login.microsoftonline.us',
		'autologon.microsoftazuread-sso.com',
		'device.login.microsoftonline.com',
		'account.activedirectory.windowsazure.com',
		'mysignins.microsoft.com',
		'fpt.microsoft.com',
		'aadcdn.msftauth.net',
		'msftauth.net',
		'msauth.net',
		'duosecurity.com',
		'okta.com',
		'pingidentity.com',
		'onelogin.com',
		'auth0.com'
	];
	function isAuthRedirectURL(raw) {
		if (!raw) return false;
		try {
			const u = new URL(String(raw));
			if (u.protocol !== 'https:') return false;
			const host = u.hostname.toLowerCase();
			return authHosts.some(d => host === d || host.endsWith('.' + d));
		} catch (_) {
			return false;
		}
	}
	function matchField(field, value, positive, state) {
		if (!value || !expected[field]) return;
		if (String(value).toLowerCase() !== String(expected[field]).toLowerCase()) {
			throw new Error('recap audio recording identity mismatch');
		}
		if (positive) state.matched = true;
	}
	function checkIdentityAgainst(raw, baseUrl, state) {
		if (!raw) return;
		let u;
		try { u = new URL(raw, baseUrl); } catch (_) { throw new Error('recap audio recording identity mismatch'); }
		const queries = [u.searchParams];
		const hashQuery = u.hash.indexOf('?');
		if (hashQuery >= 0) queries.push(new URLSearchParams(u.hash.slice(hashQuery + 1)));
		for (const params of queries) {
			for (const [key, field] of [['threadId','ThreadID'], ['driveItemId','DriveItemID'], ['driveId','DriveID'], ['callId','CallID'], ['iCalUid','ICalUID']]) {
				for (const value of params.getAll(key)) matchField(field, value, field !== 'DriveID', state);
			}
			for (const value of params.getAll('fileUrl')) {
				try {
					const filePath = new URL(value, baseUrl).pathname;
					if (/\.(mp4|mkv|webm|mov|m4a)$/i.test(filePath)) matchField('RecordingName', decodeURIComponent(filePath.split('/').pop()), true, state);
				} catch (_) { throw new Error('recap audio recording identity mismatch'); }
			}
		}
		const decoded = decodeURIComponent(raw);
		const token = decoded.match(/meeting_[a-z0-9_-]+/i);
		if (token) matchField('MeetingToken', token[0], true, state);
		const meet = decoded.match(/\/meet\/([a-z0-9_-]+)/i);
		if (meet) matchField('MeetID', meet[1], true, state);
		if (/\.(mp4|mkv|webm|mov|m4a)$/i.test(u.pathname)) matchField('RecordingName', decodeURIComponent(u.pathname.split('/').pop()), true, state);
	}
	const playBtnSelector = 'button[aria-label="Play video"], button[title="Play video"], button[data-automationid="playButton"], button[aria-label="Play"], .vjs-big-play-button, [role="button"][aria-label="Play video"]';
	function validateFrameContext(win, doc, ancestorMatched, ancestorItemId) {
		const state = { matched: false };
		const fUrl = String((win.location && win.location.href) || '');
		const fi = win.g_fileInfo || {};
		const fallbackBase = fUrl || String((typeof location !== 'undefined' && location.href) || '');
		if (fUrl) checkIdentityAgainst(fUrl, fallbackBase, state);
		matchField('DriveItemID', fi.itemId, true, state);
		matchField('DriveItemID', fi.driveItemId, true, state);
		matchField('DriveItemID', fi.VroomItemID, true, state);
		matchField('DriveItemID', fi.VroomItemId, true, state);
		matchField('RecordingName', fi.fileName, true, state);
		matchField('RecordingName', fi.name, true, state);
		let fItemId = '';
		try {
			fItemId = String(fi.itemId || fi.driveItemId || fi.VroomItemID || fi.VroomItemId || (fUrl ? new URL(fUrl).searchParams.get('driveItemId') : '') || ancestorItemId || '');
		} catch (_) {
			fItemId = String(fi.itemId || fi.driveItemId || fi.VroomItemID || fi.VroomItemId || ancestorItemId || '');
		}
		const fFileName = String(fi.fileName || fi.name || '');
		return {
			win,
			doc,
			info: fi,
			pageUrl: fUrl,
			itemId: fItemId,
			fileName: fFileName,
			baseIdentityMatched: state.matched || Boolean(ancestorMatched)
		};
	}
	function hasPlayableStreamSurface(ctx) {
		const d = ctx.doc;
		const w = ctx.win;
		const fi = ctx.info || {};
		const urlLower = String(ctx.pageUrl || '').toLowerCase();
		const media = d && d.querySelector ? d.querySelector('audio, video') : null;
		const playBtn = d && d.querySelector ? d.querySelector(playBtnSelector) : null;
		return Boolean(
			urlLower.includes('streamembed.aspx') ||
			playBtn ||
			(media && (typeof media.play === 'function' || typeof w.MediaSource !== 'undefined')) ||
			fi.transcodeUrl
		);
	}
	function frameScore(ctx) {
		let score = 0;
		const urlLower = String(ctx.pageUrl || '').toLowerCase();
		if (urlLower.includes('streamembed.aspx')) score += 100;
		if (hasPlayableStreamSurface(ctx)) score += 50;
		if (ctx.doc && ctx.doc.querySelector && ctx.doc.querySelector('audio, video')) score += 10;
		if (ctx.info && ctx.info.isDownloadBlocked) score += 5;
		const pcmEl = ctx.doc && ctx.doc.querySelector ? ctx.doc.querySelector('[data-recap-audio-pcm]') : null;
		if (pcmEl && pcmEl.hasAttribute && pcmEl.hasAttribute('data-recap-audio-pcm')) score += 20;
		if (ctx.info && (ctx.info.audioPcmBase64 || ctx.info.audioUrl || ctx.info.mediaUrl || ctx.info.downloadUrl || ctx.info.streamUrl)) score += 10;
		return score;
	}
	function collectPlayerWindows() {
		const visited = new Set();
		const contexts = [];
		function walk(win, doc, depth, ancestorMatched, ancestorItemId) {
			if (!win || depth > 3 || visited.has(win)) return;
			visited.add(win);
			let href = '';
			try { href = String((win.location && win.location.href) || ''); } catch (_) { return; }
			if (isAuthRedirectURL(href)) return;
			let targetDoc = doc;
			if (!targetDoc) {
				try { targetDoc = win.document; } catch (_) { return; }
			}
			if (!targetDoc) return;
			const frameCtx = validateFrameContext(win, targetDoc, ancestorMatched, ancestorItemId);
			contexts.push(frameCtx);
			if (depth === 3) return;
			let iframes = [];
			try {
				if (typeof targetDoc.querySelectorAll === 'function') {
					iframes = Array.from(targetDoc.querySelectorAll('iframe') || []);
				}
			} catch (_) {
				iframes = [];
			}
			for (const iframe of iframes) {
				try {
					const cw = iframe && iframe.contentWindow;
					if (!cw) continue;
					const cd = iframe.contentDocument || cw.document;
					if (!cd) continue;
					void String(cw.location && cw.location.href);
					walk(cw, cd, depth + 1, frameCtx.baseIdentityMatched, frameCtx.itemId);
				} catch (_) {
					// Cross-origin iframe; evaluated via separate CDP target.
				}
			}
		}
		walk(window, document, 0, false, '');
		try {
			if (window.parent && window.parent !== window && !visited.has(window.parent)) {
				const pHref = String((window.parent.location && window.parent.location.href) || '');
				if (pHref && !isAuthRedirectURL(pHref)) {
					validateFrameContext(window.parent, window.parent.document, false, '');
				}
			}
		} catch (_) {}
		return contexts;
	}
	function decodeAudioBufferToPCMChunks(buffer, state) {
		if (!buffer) throw new Error('recap audio decode returned empty buffer');
		const sampleRate = Number(buffer.sampleRate);
		const bufLen = Number(buffer.length);
		const duration = Number(buffer.duration);
		const outputLength = Math.ceil(bufLen * 44100 / sampleRate);
		const sampleRateRatio = sampleRate > 0 ? Math.max(1, sampleRate / 44100) : 1;
		if (!Number.isFinite(duration) || duration < 0 || !Number.isFinite(outputLength) || outputLength <= 0) {
			throw new Error(limitError);
		}
		const nextDuration = state.totalDurationSec + duration;
		const nextPCMBytes = state.totalPCMBytes + outputLength * 2;
		if (nextDuration > maxRecapAudioDurationSec ||
			bufLen * 2 > maxRecapAudioPCMBytes * sampleRateRatio ||
			nextPCMBytes > maxRecapAudioPCMBytes) {
			throw new Error(limitError);
		}
		const channels = [];
		for (let c = 0; c < buffer.numberOfChannels; c++) channels.push(buffer.getChannelData(c));
		if (!channels.length) {
			throw new Error('recap audio has no channels');
		}
		state.totalDurationSec = nextDuration;
		state.totalPCMBytes = nextPCMBytes;
		for (let start = 0; start < outputLength; start += 65536) {
			const count = Math.min(65536, outputLength - start);
			const pcm = new Int16Array(count);
			for (let i = 0; i < count; i++) {
				const index = Math.min(bufLen - 1, Math.floor((start + i) * sampleRate / 44100));
				let sample = 0;
				for (const channel of channels) sample += channel[index];
				sample = Math.max(-1, Math.min(1, sample / channels.length));
				pcm[i] = sample < 0 ? sample * 32768 : sample * 32767;
			}
			const bytes = new Uint8Array(pcm.buffer);
			let binary = '';
			for (let i = 0; i < bytes.length; i += 8192) binary += String.fromCharCode(...bytes.subarray(i, i + 8192));
			const b64 = typeof btoa === 'function' ? btoa(binary) : Buffer.from(binary, 'binary').toString('base64');
			state.pcmChunks.push(b64);
		}
	}

	const playerContexts = collectPlayerWindows();
	playerContexts.sort((a, b) => frameScore(b) - frameScore(a));
	const playerCtx = playerContexts[0];
	const playerWin = playerCtx.win;
	const playerDoc = playerCtx.doc;
	const info = playerCtx.info;
	const pageUrl = playerCtx.pageUrl;
	const itemId = playerCtx.itemId;
	const fileName = playerCtx.fileName;
	const baseIdentityMatched = playerCtx.baseIdentityMatched;

	const pcmEl = playerDoc.querySelector ? playerDoc.querySelector('[data-recap-audio-pcm]') : null;
	const hasPCM = pcmEl && pcmEl.hasAttribute && pcmEl.hasAttribute('data-recap-audio-pcm');
	if (hasPCM && enforceIdentity && !baseIdentityMatched) throw new Error('recap audio recording identity unverified');
	const supplied = (hasPCM && pcmEl.getAttribute('data-recap-audio-pcm')) ||
		((!enforceIdentity || baseIdentityMatched) && info.audioPcmBase64);
	if (supplied) {
		if (enforceIdentity && !baseIdentityMatched) throw new Error('recap audio recording identity unverified');
		const pcm = String(supplied);
		const padding = pcm.endsWith('==') ? 2 : (pcm.endsWith('=') ? 1 : 0);
		if (Math.floor(pcm.length / 4) * 3 - padding > maxRecapAudioPCMBytes) throw new Error(limitError);
		return JSON.stringify({pageUrl, itemId, fileName, pcmChunks: [pcm]});
	}

	const media = playerDoc.querySelector ? playerDoc.querySelector('audio, video') : null;
	const source = media && media.querySelector ? media.querySelector('source') : null;
	const rawCandidates = [
		media && media.getAttribute ? media.getAttribute('data-audio-src') : '',
		source && source.src,
		info.audioUrl,
		info.mediaUrl,
		info.downloadUrl,
		info.streamUrl,
		media && media.currentSrc,
		media && media.src,
	];
	const nonBlobCandidates = [];
	const blobCandidates = [];
	const seenCandidates = new Set();
	for (const raw of rawCandidates) {
		const u = String(raw || '').trim();
		if (!u || seenCandidates.has(u)) continue;
		seenCandidates.add(u);
		if (/^blob:/i.test(u)) blobCandidates.push(u);
		else nonBlobCandidates.push(u);
	}
	const candidates = nonBlobCandidates.concat(blobCandidates);
	const streamSurface = hasPlayableStreamSurface(playerCtx);
	if (!candidates.length && !streamSurface) return '';

	let blockedErr = null;
	let lastFetchErr = null;
	const fetchFn = (typeof playerWin.fetch === 'function' ? playerWin.fetch : (typeof fetch === 'function' ? fetch : null));
	const getAudioCtxCtor = () => {
		if (typeof playerWin.AudioContext === 'function') return playerWin.AudioContext;
		if (typeof AudioContext === 'function') return AudioContext;
		return null;
	};

	for (const candidateUrl of candidates) {
		const candState = { matched: false };
		checkIdentityAgainst(candidateUrl, pageUrl, candState);
		const candidateHasIdentity = candState.matched;
		if (enforceIdentity && !(baseIdentityMatched || candidateHasIdentity)) {
			lastFetchErr = new Error('recap audio recording identity unverified');
			continue;
		}
		for (const rawDuration of [media && media.duration, info.duration, info.durationSeconds]) {
			const duration = Number(rawDuration);
			if (Number.isFinite(duration) && duration > 0 &&
				(duration > maxRecapAudioDurationSec || Math.ceil(duration * 44100) * 2 > maxRecapAudioPCMBytes)) throw new Error(limitError);
		}
		if (!fetchFn) {
			lastFetchErr = new Error('recap audio fetch unavailable');
			continue;
		}
		let resp = null;
		try {
			resp = await fetchFn.call(playerWin, candidateUrl, {credentials: 'include', redirect: 'follow'});
		} catch (err) {
			if (err && err.name === 'TypeError' && !/^blob:/i.test(candidateUrl)) {
				try {
					resp = await fetchFn.call(playerWin, candidateUrl, {credentials: 'omit', redirect: 'follow'});
				} catch (retryErr) {
					if (info.isDownloadBlocked) {
						blockedErr = new Error('recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)');
					}
					lastFetchErr = retryErr;
					continue;
				}
			} else {
				if (info.isDownloadBlocked) {
					blockedErr = new Error('recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)');
				}
				lastFetchErr = err;
				continue;
			}
		}
		if (!resp || !resp.ok) {
			if (info.isDownloadBlocked) {
				blockedErr = new Error('recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)');
				lastFetchErr = blockedErr;
				continue;
			}
			lastFetchErr = new Error('recap audio fetch failed: ' + (resp ? resp.status : 'network error'));
			continue;
		}
		const ct = String((resp.headers && resp.headers.get && resp.headers.get('content-type')) || '').toLowerCase();
		if (ct.includes('text/html')) {
			const err = new Error(info.isDownloadBlocked
				? 'recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)'
				: 'recap audio fetch returned unexpected text/html content');
			if (info.isDownloadBlocked) blockedErr = err;
			lastFetchErr = err;
			continue;
		}
		let respHasIdentity = false;
		if (resp.url) {
			const respState = { matched: false };
			checkIdentityAgainst(resp.url, pageUrl, respState);
			respHasIdentity = respState.matched;
		}
		if (enforceIdentity && !(baseIdentityMatched || candidateHasIdentity || respHasIdentity)) {
			lastFetchErr = new Error('recap audio recording identity unverified');
			continue;
		}
		const selectedUrl = (resp.url && respHasIdentity) ? resp.url : (candidateHasIdentity ? candidateUrl : (resp.url || candidateUrl));
		const length = Number(resp.headers && resp.headers.get && resp.headers.get('Content-Length'));
		if (Number.isFinite(length) && length > maxRecapAudioSourceBytes) throw new Error(limitError);
		if (!resp.body || !resp.body.getReader) {
			lastFetchErr = new Error('recap audio streaming unavailable');
			continue;
		}
		const reader = resp.body.getReader();
		const parts = [];
		let sourceBytes = 0;
		let readErr = null;
		while (true) {
			let part;
			try {
				part = await reader.read();
			} catch (err) {
				readErr = err;
				break;
			}
			if (part.done) break;
			sourceBytes += part.value.byteLength;
			if (sourceBytes > maxRecapAudioSourceBytes) {
				await reader.cancel();
				throw new Error(limitError);
			}
			parts.push(part.value);
		}
		if (readErr) {
			lastFetchErr = readErr;
			continue;
		}
		const sourceData = new Uint8Array(sourceBytes);
		let offset = 0;
		for (const part of parts) { sourceData.set(part, offset); offset += part.byteLength; }
		const AudioCtxCtor = getAudioCtxCtor();
		if (!AudioCtxCtor) {
			lastFetchErr = new Error('recap audio AudioContext unavailable');
			continue;
		}
		const audioContext = new AudioCtxCtor({sampleRate: 44100});
		try {
			let buffer = null;
			try {
				buffer = await audioContext.decodeAudioData(sourceData.buffer);
			} catch (decodeErr) {
				if (decodeErr && String(decodeErr.message || decodeErr).includes(limitError)) throw decodeErr;
				lastFetchErr = decodeErr;
				continue;
			}
			const pcmState = { totalDurationSec: 0, totalPCMBytes: 0, pcmChunks: [] };
			try {
				decodeAudioBufferToPCMChunks(buffer, pcmState);
			} catch (pcmErr) {
				const pcmMsg = String((pcmErr && pcmErr.message) || pcmErr || '');
				if (pcmMsg.includes(limitError)) throw pcmErr;
				lastFetchErr = pcmErr;
				continue;
			}
			return JSON.stringify({pageUrl, itemId, fileName, mediaUrl: selectedUrl, sourceBytes, durationSec: pcmState.totalDurationSec, pcmChunks: pcmState.pcmChunks});
		} finally { await audioContext.close(); }
	}

	function readU32(u8, off) {
		return ((u8[off] << 24) | (u8[off + 1] << 16) | (u8[off + 2] << 8) | u8[off + 3]) >>> 0;
	}
	function readU64(u8, off) {
		return readU32(u8, off) * 0x100000000 + readU32(u8, off + 4);
	}
	function readFourCC(u8, off) {
		if (!u8 || off + 4 > u8.length) return '';
		return String.fromCharCode(u8[off], u8[off + 1], u8[off + 2], u8[off + 3]);
	}
	function isFMP4InitSegment(u8) {
		if (!u8 || u8.length < 8) return false;
		const t = readFourCC(u8, 4);
		return t === 'ftyp' || t === 'moov';
	}
	function isFMP4MediaSegment(u8) {
		if (!u8 || u8.length < 8) return false;
		const t = readFourCC(u8, 4);
		return t === 'styp' || t === 'sidx' || t === 'moof' || t === 'emsg';
	}
	function readBoxHeader(u8, pos, end) {
		if (pos + 8 > end) return null;
		let boxSize = readU32(u8, pos);
		const type = readFourCC(u8, pos + 4);
		let hdrSize = 8;
		if (boxSize === 1) {
			if (pos + 16 > end) return null;
			boxSize = readU64(u8, pos + 8);
			hdrSize = 16;
			if (!Number.isFinite(boxSize) || boxSize < 16) return null;
		} else if (boxSize === 0) {
			boxSize = end - pos;
		} else if (boxSize < 8) {
			return null;
		}
		if (pos + boxSize > end) return null;
		return { boxSize, hdrSize, type };
	}
	function classifyInitSegmentTrack(u8) {
		if (!u8 || u8.length < 8) return 'unknown';
		let hasAudio = false;
		let hasVideo = false;
		const containerBoxes = new Set(['moov', 'trak', 'mdia', 'minf', 'stbl']);
		function walk(start, end) {
			let pos = start;
			while (pos + 8 <= end) {
				const h = readBoxHeader(u8, pos, end);
				if (!h) break;
				const bodyStart = pos + h.hdrSize;
				const boxEnd = pos + h.boxSize;
				if (h.type === 'hdlr' && bodyStart + 12 <= boxEnd) {
					const ht = readFourCC(u8, bodyStart + 8);
					if (ht === 'soun') hasAudio = true;
					else if (ht === 'vide') hasVideo = true;
				} else if (h.type === 'stsd' && bodyStart + 8 <= boxEnd) {
					walk(bodyStart + 8, boxEnd);
				} else if (h.type === 'mp4a') {
					hasAudio = true;
				} else if (h.type === 'avc1' || h.type === 'hev1' || h.type === 'hvc1' || h.type === 'vp09' || h.type === 'av01') {
					hasVideo = true;
				} else if (containerBoxes.has(h.type)) {
					walk(bodyStart, boxEnd);
				}
				pos = boxEnd;
			}
		}
		walk(0, u8.length);
		if (hasAudio && !hasVideo) return 'audio';
		if (hasVideo && !hasAudio) return 'video';
		for (let i = 0; i + 4 <= u8.length; i++) {
			const cc = readFourCC(u8, i);
			if (cc === 'mp4a') hasAudio = true;
			else if (cc === 'avc1' || cc === 'hev1' || cc === 'hvc1' || cc === 'vp09' || cc === 'av01') hasVideo = true;
		}
		if (hasAudio && !hasVideo) return 'audio';
		if (hasVideo && !hasAudio) return 'video';
		return hasAudio ? 'audio' : 'unknown';
	}
	function parseFMP4DecodeTime(u8) {
		if (!u8 || u8.length < 16) return null;
		let sidxTime = null;
		function walkTraf(start, end) {
			let pos = start;
			while (pos + 8 <= end) {
				const h = readBoxHeader(u8, pos, end);
				if (!h) break;
				const bodyStart = pos + h.hdrSize;
				const boxEnd = pos + h.boxSize;
				if (h.type === 'tfdt' && bodyStart + 8 <= boxEnd) {
					const ver = u8[bodyStart];
					if (ver === 1 && bodyStart + 12 <= boxEnd) return readU64(u8, bodyStart + 4);
					return readU32(u8, bodyStart + 4);
				}
				pos = boxEnd;
			}
			return null;
		}
		function walkMoof(start, end) {
			let pos = start;
			while (pos + 8 <= end) {
				const h = readBoxHeader(u8, pos, end);
				if (!h) break;
				const bodyStart = pos + h.hdrSize;
				const boxEnd = pos + h.boxSize;
				if (h.type === 'traf') {
					const dt = walkTraf(bodyStart, boxEnd);
					if (dt !== null) return dt;
				}
				pos = boxEnd;
			}
			return null;
		}
		let pos = 0;
		while (pos + 8 <= u8.length) {
			const h = readBoxHeader(u8, pos, u8.length);
			if (!h) break;
			const bodyStart = pos + h.hdrSize;
			const boxEnd = pos + h.boxSize;
			if (h.type === 'moof') {
				const dt = walkMoof(bodyStart, boxEnd);
				if (dt !== null) return dt;
			} else if (h.type === 'sidx' && sidxTime === null && bodyStart + 16 <= boxEnd) {
				const ver = u8[bodyStart];
				if (ver === 1 && bodyStart + 20 <= boxEnd) sidxTime = readU64(u8, bodyStart + 12);
				else if (ver === 0) sidxTime = readU32(u8, bodyStart + 12);
			}
			pos = boxEnd;
		}
		return sidxTime;
	}
	function zeroFMP4DecodeTime(u8) {
		const copy = new Uint8Array(u8);
		function walkBox(start, end) {
			let pos = start;
			while (pos + 8 <= end) {
				const h = readBoxHeader(copy, pos, end);
				if (!h) break;
				const bodyStart = pos + h.hdrSize;
				const boxEnd = pos + h.boxSize;
				if (h.type === 'moof' || h.type === 'traf') {
					walkBox(bodyStart, boxEnd);
				} else if (h.type === 'tfdt' && bodyStart + 8 <= boxEnd) {
					const ver = copy[bodyStart];
					const len = (ver === 1 && bodyStart + 12 <= boxEnd) ? 8 : 4;
					for (let i = 0; i < len; i++) copy[bodyStart + 4 + i] = 0;
				} else if (h.type === 'sidx' && bodyStart + 16 <= boxEnd) {
					const ver = copy[bodyStart];
					const len = (ver === 1 && bodyStart + 20 <= boxEnd) ? 8 : 4;
					for (let i = 0; i < len; i++) copy[bodyStart + 12 + i] = 0;
				}
				pos = boxEnd;
			}
		}
		walkBox(0, copy.length);
		return copy;
	}
	function sanitizeMediaUrl(raw) {
		if (!raw) return '';
		try {
			const u = new URL(String(raw), pageUrl || String((typeof location !== 'undefined' && location.href) || ''));
			for (const k of ['tempauth', 'access_token', 'sig', 'se', 'sp', 'sv', 'auth']) {
				u.searchParams.delete(k);
			}
			return u.toString();
		} catch (_) {
			return '';
		}
	}

	async function captureStreamEmbedAudio() {
		for (const rawDuration of [media && media.duration, info.duration, info.durationSeconds]) {
			const d = Number(rawDuration);
			if (Number.isFinite(d) && d > 0 &&
				(d > maxRecapAudioDurationSec || Math.ceil(d * 44100) * 2 > maxRecapAudioPCMBytes)) {
				throw new Error(limitError);
			}
		}
		const abortCtrl = typeof AbortController === 'function' ? new AbortController() : null;
		let fatalErr = null;
		let streamSourceBytes = 0;
		let pathAActive = false;
		let pathAStarted = false;
		let pathAFailed = false;
		let activityObserved = false;
		let transcodeAudioTemplateUrl = info.transcodeUrl ? String(info.transcodeUrl) : '';
		let transcodeOrigin = '';
		let transcodeWsd = 0;
		let transcodePpd = 0;
		const capturedAuthHeaders = {};
		const allowlistedHeaderNames = ['accept', 'authorization', 'prefer'];
		let cryptoInitSegment = null;
		let audioCryptoKeyInfo = null;
		let fallbackCryptoKeyInfo = null;
		let mseInitSegment = null;
		const mseSegments = new Map();
		let mseSeq = 0;
		let mseLastAppendAt = 0;
		let mseMaxBufferedEnd = 0;
		let audioSBRef = null;
		let activeTimers = new Set();
		const trackedListeners = [];
		const restorers = [];
		let hooksRestored = false;
		let pathAPromise = null;
		let pathAStartedAt = 0;
		let wakeLoop = null;

		function signalWake() {
			if (wakeLoop) {
				const fn = wakeLoop;
				wakeLoop = null;
				fn();
			}
		}
		function sleepOrWake(ms) {
			return new Promise(resolve => {
				let done = false;
				const finish = () => {
					if (done) return;
					done = true;
					clearTimeout(tid);
					activeTimers.delete(tid);
					if (wakeLoop === finish) wakeLoop = null;
					resolve();
				};
				const tid = setTimeout(finish, ms);
				activeTimers.add(tid);
				wakeLoop = finish;
			});
		}
		function toUint8Array(data) {
			if (!data) return null;
			if (data instanceof Uint8Array) return data;
			if (data instanceof ArrayBuffer) return new Uint8Array(data);
			if (ArrayBuffer.isView(data)) return new Uint8Array(data.buffer, data.byteOffset, data.byteLength);
			return null;
		}
		function mseBytesTotal() {
			let total = mseInitSegment ? mseInitSegment.byteLength : 0;
			for (const entry of mseSegments.values()) total += entry.data.byteLength;
			return total;
		}
		function recordMSESegment(u8) {
			if (!u8 || u8.byteLength === 0 || fatalErr) return;
			if (isFMP4InitSegment(u8)) {
				if (classifyInitSegmentTrack(u8) === 'video') return;
				if (!mseInitSegment) {
					if (streamSourceBytes + u8.byteLength > maxRecapAudioSourceBytes) {
						fatalErr = new Error(limitError);
						if (abortCtrl) abortCtrl.abort();
						signalWake();
						return;
					}
					streamSourceBytes += u8.byteLength;
					mseInitSegment = new Uint8Array(u8);
				}
				activityObserved = true;
				mseLastAppendAt = Date.now();
				signalWake();
				return;
			}
			if (!isFMP4MediaSegment(u8)) return;
			const dts = parseFMP4DecodeTime(u8);
			const key = (dts !== null && dts !== undefined && Number.isFinite(dts)) ? ('dts:' + dts) : ('seq:' + (mseSeq++));
			if (mseSegments.has(key)) {
				activityObserved = true;
				mseLastAppendAt = Date.now();
				return;
			}
			if (streamSourceBytes + u8.byteLength > maxRecapAudioSourceBytes) {
				fatalErr = new Error(limitError);
				if (abortCtrl) abortCtrl.abort();
				signalWake();
				return;
			}
			streamSourceBytes += u8.byteLength;
			mseSegments.set(key, { dts, seq: mseSeq++, data: new Uint8Array(u8) });
			activityObserved = true;
			mseLastAppendAt = Date.now();
			signalWake();
		}
		function createVideoSourceBufferShim() {
			const ET = playerWin.EventTarget || EventTarget;
			const shim = new ET();
			let shimBufferedEnd = 0;
			shim.__isRecapVideoShim = true;
			shim.updating = false;
			shim.mode = 'segments';
			shim.timestampOffset = 0;
			shim.appendWindowStart = 0;
			shim.appendWindowEnd = Infinity;
			shim.buffered = {
				get length() { return shimBufferedEnd > 0 ? 1 : 0; },
				start: () => 0,
				end: () => shimBufferedEnd
			};
			function fireAsyncUpdate() {
				shim.updating = true;
				const tid = setTimeout(() => {
					activeTimers.delete(tid);
					shim.updating = false;
					try {
						const Ev = playerWin.Event || Event;
						shim.dispatchEvent(new Ev('updatestart'));
						shim.dispatchEvent(new Ev('update'));
						shim.dispatchEvent(new Ev('updateend'));
					} catch (_) {}
				}, 0);
				activeTimers.add(tid);
			}
			shim.appendBuffer = (data) => {
				if (mseMaxBufferedEnd > shimBufferedEnd) shimBufferedEnd = mseMaxBufferedEnd;
				else if (shimBufferedEnd === 0 && data && data.byteLength) shimBufferedEnd = 0.1;
				fireAsyncUpdate();
			};
			shim.remove = () => { fireAsyncUpdate(); };
			shim.abort = () => {};
			shim.changeType = () => {};
			return shim;
		}
		function stepVideoPlayback() {
			if (pathAActive || fatalErr) return;
			const m = playerDoc.querySelector ? playerDoc.querySelector('audio, video') : null;
			if (!m) return;
			try {
				m.muted = true;
				m.volume = 0;
				if (m.playbackRate !== 16) m.playbackRate = 16;
			} catch (_) {}
			const sb = audioSBRef;
			if (sb && sb.buffered && sb.buffered.length > 0) {
				try {
					const bufEnd = sb.buffered.end(sb.buffered.length - 1);
					if (Number.isFinite(bufEnd) && bufEnd > 0) {
						if (bufEnd > mseMaxBufferedEnd) mseMaxBufferedEnd = bufEnd;
						const targetTime = Math.max(0, bufEnd - 0.05);
						if (Number.isFinite(m.currentTime) && targetTime > m.currentTime + 0.01) {
							m.currentTime = targetTime;
						}
						if (bufEnd > 30 && !sb.updating && typeof sb.remove === 'function') {
							try { sb.remove(0, bufEnd - 30); } catch (_) {}
						}
					}
				} catch (_) {}
			}
		}
		function normalizeHeaders(hdrs) {
			const out = {};
			if (!hdrs) return out;
			if (typeof hdrs.forEach === 'function') {
				hdrs.forEach((v, k) => { out[String(k).toLowerCase()] = String(v); });
			} else if (Array.isArray(hdrs)) {
				for (const pair of hdrs) {
					if (Array.isArray(pair) && pair.length >= 2) out[String(pair[0]).toLowerCase()] = String(pair[1]);
				}
			} else if (typeof hdrs === 'object') {
				for (const [k, v] of Object.entries(hdrs)) out[String(k).toLowerCase()] = String(v);
			}
			return out;
		}
		function inspectTranscodeRequest(rawUrl, headersObj, hookWin) {
			if (!rawUrl) return { isVideoSegment: false };
			let u;
			try {
				const base = (hookWin && hookWin.location && hookWin.location.href) || pageUrl || String(location.href || '');
				u = new URL(String(rawUrl), base);
			} catch (_) {
				return { isVideoSegment: false };
			}
			const part = (u.searchParams.get('part') || '').toLowerCase();
			const track = (u.searchParams.get('track') || '').toLowerCase();
			const isTranscode = u.pathname.toLowerCase().includes('onedrive.transcode') || part === 'initsegment' || part === 'mediasegment';
			if (!isTranscode) return { isVideoSegment: false };
			let winOrigin = '';
			try {
				winOrigin = (hookWin && hookWin.location && hookWin.location.origin) || new URL(pageUrl).origin;
			} catch (_) {
				winOrigin = '';
			}
			const sameOrigin = Boolean(winOrigin) && u.origin === winOrigin && (u.protocol === 'https:' || u.protocol === 'http:');
			if (!sameOrigin) return { isVideoSegment: false };
			if (headersObj) {
				for (const [k, v] of Object.entries(headersObj)) {
					const kl = String(k).toLowerCase();
					if (v && (allowlistedHeaderNames.includes(kl) || kl.startsWith('x-ms-'))) {
						capturedAuthHeaders[kl] = String(v);
					}
				}
			}
			const wsd = Number(u.searchParams.get('wsd'));
			const ppd = Number(u.searchParams.get('ppd'));
			if (Number.isFinite(wsd) && wsd > 0) transcodeWsd = wsd;
			if (Number.isFinite(ppd) && ppd > 0) {
				transcodePpd = ppd;
				const ppdSec = ppd >= 1e6 ? (ppd / 1e7) : ppd;
				if (Number.isFinite(ppdSec) && (ppdSec > maxRecapAudioDurationSec || Math.ceil(ppdSec * 44100) * 2 > maxRecapAudioPCMBytes)) {
					fatalErr = new Error(limitError);
					if (abortCtrl) abortCtrl.abort();
					signalWake();
				}
			}
			if (!transcodeAudioTemplateUrl || track === 'audio') {
				const tmpl = new URL(u.toString());
				tmpl.searchParams.set('track', 'audio');
				tmpl.searchParams.set('quality', 'audcopy');
				tmpl.searchParams.delete('segmentTime');
				tmpl.searchParams.delete('part');
				transcodeAudioTemplateUrl = tmpl.toString();
				transcodeOrigin = tmpl.origin;
			}
			activityObserved = true;
			signalWake();
			maybeStartPathA();
			return { isVideoSegment: track === 'video' && part === 'mediasegment' && pathAActive, parsedUrl: u };
		}
		function maybeStartPathA() {
			if (pathAStarted || pathAFailed || fatalErr || pathAPromise) return;
			if (!transcodeAudioTemplateUrl || !(transcodeWsd > 0) || !(transcodePpd > 0)) return;
			const keyInfo = audioCryptoKeyInfo || fallbackCryptoKeyInfo;
			if (!keyInfo && !mseInitSegment && !mseSegments.size) return;
			pathAStarted = true;
			pathAStartedAt = Date.now();
			pathAPromise = runPathA(keyInfo).catch(err => {
				const msg = String((err && err.message) || err || '');
				if (msg.includes(limitError) || msg.includes('identity mismatch') || msg.includes('identity unverified')) {
					fatalErr = err;
					if (abortCtrl) abortCtrl.abort();
					signalWake();
					return null;
				}
				pathAActive = false;
				pathAFailed = true;
				streamSourceBytes = mseBytesTotal();
				const m = playerDoc.querySelector ? playerDoc.querySelector('audio, video') : null;
				if (m && typeof m.play === 'function') {
					try {
						m.muted = true;
						m.volume = 0;
						m.playbackRate = 16;
						Promise.resolve(m.play()).catch(() => {});
					} catch (_) {}
				}
				signalWake();
				return null;
			});
		}
		async function fetchBytesWithCap(urlStr, origFetchFn, hookWin) {
			if (fatalErr) throw fatalErr;
			const u = new URL(urlStr);
			const hdrs = (transcodeOrigin && u.origin === transcodeOrigin && Object.keys(capturedAuthHeaders).length)
				? Object.assign({}, capturedAuthHeaders)
				: undefined;
			const fetchOpts = {
				credentials: 'include',
				signal: abortCtrl ? abortCtrl.signal : undefined,
				headers: hdrs
			};
			const resp = await origFetchFn.call(hookWin, urlStr, fetchOpts);
			if (!resp || !resp.ok) {
				throw new Error('transcode fetch failed: ' + (resp ? resp.status : 'network error'));
			}
			const cl = Number(resp.headers && resp.headers.get && resp.headers.get('content-length'));
			if (Number.isFinite(cl) && cl > 0 && streamSourceBytes + cl > maxRecapAudioSourceBytes) {
				fatalErr = new Error(limitError);
				if (abortCtrl) abortCtrl.abort();
				throw fatalErr;
			}
			if (!resp.body || !resp.body.getReader) {
				const ab = await resp.arrayBuffer();
				const u8 = new Uint8Array(ab);
				if (streamSourceBytes + u8.byteLength > maxRecapAudioSourceBytes) {
					fatalErr = new Error(limitError);
					if (abortCtrl) abortCtrl.abort();
					throw fatalErr;
				}
				streamSourceBytes += u8.byteLength;
				return u8;
			}
			const reader = resp.body.getReader();
			const chunks = [];
			let total = 0;
			while (true) {
				const part = await reader.read();
				if (part.done) break;
				const v = part.value;
				if (!v || !v.byteLength) continue;
				if (streamSourceBytes + v.byteLength > maxRecapAudioSourceBytes) {
					try { await reader.cancel(); } catch (_) {}
					fatalErr = new Error(limitError);
					if (abortCtrl) abortCtrl.abort();
					throw fatalErr;
				}
				streamSourceBytes += v.byteLength;
				total += v.byteLength;
				chunks.push(v);
			}
			const out = new Uint8Array(total);
			let off = 0;
			for (const c of chunks) { out.set(c, off); off += c.byteLength; }
			return out;
		}
		async function decryptSegmentBytes(rawU8, keyInfo, origDecryptFn, subtleCtx) {
			if (isFMP4InitSegment(rawU8) || isFMP4MediaSegment(rawU8)) return rawU8;
			if (!keyInfo || !origDecryptFn || !subtleCtx) {
				throw new Error('recap audio missing WebCrypto key for encrypted segment');
			}
			if (!keyInfo.prefixIV && keyInfo.iv) {
				try {
					const ab = await origDecryptFn.call(subtleCtx, { name: keyInfo.algorithm, iv: keyInfo.iv }, keyInfo.key, rawU8);
					const dec = new Uint8Array(ab);
					if (isFMP4InitSegment(dec) || isFMP4MediaSegment(dec)) return dec;
				} catch (_) {}
			}
			if (rawU8.byteLength > 32) {
				const iv = rawU8.subarray(0, 16);
				const ct = rawU8.subarray(16);
				const ab = await origDecryptFn.call(subtleCtx, { name: keyInfo.algorithm, iv }, keyInfo.key, ct);
				const dec = new Uint8Array(ab);
				if (isFMP4InitSegment(dec) || isFMP4MediaSegment(dec)) {
					keyInfo.prefixIV = true;
					return dec;
				}
			}
			throw new Error('decrypted segment is not valid fMP4');
		}
		let origPlayerFetch = (typeof playerWin.fetch === 'function' ? playerWin.fetch : (typeof fetch === 'function' ? fetch : null));
		let origPlayerDecrypt = (playerWin.crypto && playerWin.crypto.subtle && playerWin.crypto.subtle.decrypt) || null;
		let origPlayerSubtle = (playerWin.crypto && playerWin.crypto.subtle) || null;
		let origPlayerFetchWin = playerWin;

		async function runPathA(keyInfo) {
			const tState = { matched: false };
			checkIdentityAgainst(transcodeAudioTemplateUrl, pageUrl, tState);
			if (enforceIdentity && !baseIdentityMatched && !tState.matched) {
				throw new Error('recap audio recording identity unverified');
			}
			const ppdSec = transcodePpd >= 1e6 ? (transcodePpd / 1e7) : transcodePpd;
			if (!Number.isFinite(ppdSec) || ppdSec <= 0 || ppdSec > maxRecapAudioDurationSec || Math.ceil(ppdSec * 44100) * 2 > maxRecapAudioPCMBytes) {
				throw new Error(limitError);
			}
			const totalSegments = Math.ceil(transcodePpd / transcodeWsd);
			if (!Number.isFinite(totalSegments) || totalSegments < 1 || totalSegments > 2400) {
				throw new Error('transcode segment count out of bounds');
			}
			pathAActive = true;
			const m = playerDoc.querySelector ? playerDoc.querySelector('audio, video') : null;
			if (m && typeof m.pause === 'function') {
				try { m.pause(); } catch (_) {}
			}
			let initSeg = null;
			if (cryptoInitSegment) {
				if (streamSourceBytes + cryptoInitSegment.byteLength > maxRecapAudioSourceBytes) {
					throw new Error(limitError);
				}
				streamSourceBytes += cryptoInitSegment.byteLength;
				initSeg = cryptoInitSegment;
			} else if (mseInitSegment) {
				initSeg = mseInitSegment;
			} else {
				const initUrl = new URL(transcodeAudioTemplateUrl);
				initUrl.searchParams.set('part', 'initsegment');
				initUrl.searchParams.set('track', 'audio');
				initUrl.searchParams.set('quality', 'audcopy');
				const rawInit = await fetchBytesWithCap(initUrl.toString(), origPlayerFetch, origPlayerFetchWin);
				const decInit = await decryptSegmentBytes(rawInit, keyInfo, origPlayerDecrypt, origPlayerSubtle);
				if (!isFMP4InitSegment(decInit) || classifyInitSegmentTrack(decInit) === 'video') {
					throw new Error('transcode initsegment is not audio fMP4');
				}
				initSeg = decInit;
			}
			const segments = new Array(totalSegments);
			let nextIdx = 0;
			async function worker() {
				while (nextIdx < totalSegments) {
					if (fatalErr || (abortCtrl && abortCtrl.signal.aborted)) return;
					const k = nextIdx++;
					const segUrl = new URL(transcodeAudioTemplateUrl);
					segUrl.searchParams.set('part', 'mediasegment');
					segUrl.searchParams.set('track', 'audio');
					segUrl.searchParams.set('quality', 'audcopy');
					segUrl.searchParams.set('segmentTime', String(k * transcodeWsd));
					const rawSeg = await fetchBytesWithCap(segUrl.toString(), origPlayerFetch, origPlayerFetchWin);
					const decSeg = await decryptSegmentBytes(rawSeg, keyInfo, origPlayerDecrypt, origPlayerSubtle);
					if (!isFMP4MediaSegment(decSeg)) {
						throw new Error('transcode mediasegment is not fMP4');
					}
					segments[k] = decSeg;
				}
			}
			const workers = [];
			for (let i = 0; i < Math.min(16, totalSegments); i++) workers.push(worker());
			await Promise.all(workers);
			if (fatalErr) throw fatalErr;
			if (segments.some(s => !s)) throw new Error('incomplete Path A segment set');
			return { initSegment: initSeg, mediaSegments: segments, selectedMediaUrl: transcodeAudioTemplateUrl };
		}

		function installWindowHooks(w) {
			if (!w) return;
			if (w.crypto && w.crypto.subtle && typeof w.crypto.subtle.decrypt === 'function') {
				const subtle = w.crypto.subtle;
				const origDecrypt = subtle.decrypt;
				const hadOwn = Object.prototype.hasOwnProperty.call(subtle, 'decrypt');
				const origDesc = hadOwn ? Object.getOwnPropertyDescriptor(subtle, 'decrypt') : null;
				if (w === playerWin || !origPlayerDecrypt) {
					origPlayerDecrypt = origDecrypt;
					origPlayerSubtle = subtle;
				}
				const wrappedDecrypt = async function(algorithm, key, data) {
					const algoName = (algorithm && typeof algorithm === 'object' ? String(algorithm.name || '') : String(algorithm || '')).toUpperCase();
					let ivClone = null;
					if (algorithm && algorithm.iv) {
						try {
							const ivView = toUint8Array(algorithm.iv);
							if (ivView) ivClone = new Uint8Array(ivView);
						} catch (_) {}
					}
					const plainBuf = await origDecrypt.call(this, algorithm, key, data);
					try {
						const u8 = new Uint8Array(plainBuf);
						if (u8.byteLength > maxRecapAudioSourceBytes) {
							fatalErr = new Error(limitError);
							if (abortCtrl) abortCtrl.abort();
							signalWake();
						} else if (isFMP4InitSegment(u8)) {
							if (classifyInitSegmentTrack(u8) === 'audio') {
								if (!cryptoInitSegment) cryptoInitSegment = new Uint8Array(u8);
								audioCryptoKeyInfo = { algorithm: algoName || 'AES-CBC', key, iv: ivClone, prefixIV: false };
								activityObserved = true;
								signalWake();
								maybeStartPathA();
							}
						} else if (isFMP4MediaSegment(u8)) {
							if (!audioCryptoKeyInfo && !fallbackCryptoKeyInfo) {
								fallbackCryptoKeyInfo = { algorithm: algoName || 'AES-CBC', key, iv: ivClone, prefixIV: false };
							}
							activityObserved = true;
							signalWake();
							maybeStartPathA();
						}
					} catch (_) {}
					return plainBuf;
				};
				try {
					Object.defineProperty(subtle, 'decrypt', { value: wrappedDecrypt, configurable: true, writable: true });
					restorers.push(() => {
						try {
							if (hadOwn && origDesc) Object.defineProperty(subtle, 'decrypt', origDesc);
							else delete subtle.decrypt;
						} catch (_) { subtle.decrypt = origDecrypt; }
					});
				} catch (_) {}
			}
			if (typeof w.fetch === 'function') {
				const origFetch = w.fetch;
				if (w === playerWin || !origPlayerFetch) {
					origPlayerFetch = origFetch;
					origPlayerFetchWin = w;
				}
				w.fetch = function(input, init) {
					const reqUrl = (input && typeof input === 'object' && 'url' in input) ? input.url : String(input || '');
					const hdrs = Object.assign(
						{},
						(input && typeof input === 'object' && input.headers) ? normalizeHeaders(input.headers) : {},
						(init && init.headers) ? normalizeHeaders(init.headers) : {}
					);
					const reqInfo = inspectTranscodeRequest(reqUrl, hdrs, w);
					if (reqInfo.isVideoSegment) {
						return Promise.resolve({
							ok: true,
							status: 204,
							url: String(reqInfo.parsedUrl || reqUrl),
							headers: { get: () => null },
							body: { getReader: () => ({ read: async () => ({ done: true }), cancel: async () => {} }) },
							arrayBuffer: async () => new ArrayBuffer(0)
						});
					}
					return origFetch.apply(this, arguments);
				};
				restorers.push(() => { w.fetch = origFetch; });
			}
			if (w.XMLHttpRequest && w.XMLHttpRequest.prototype) {
				const proto = w.XMLHttpRequest.prototype;
				const origOpen = proto.open;
				const origSetHeader = proto.setRequestHeader;
				const origSend = proto.send;
				const xhrMeta = new WeakMap();
				proto.open = function(method, url) {
					xhrMeta.set(this, { method: String(method || 'GET'), url: String(url || ''), headers: {} });
					return origOpen.apply(this, arguments);
				};
				proto.setRequestHeader = function(name, value) {
					const meta = xhrMeta.get(this);
					if (meta && name) meta.headers[String(name).toLowerCase()] = String(value);
					return origSetHeader.apply(this, arguments);
				};
				proto.send = function() {
					const meta = xhrMeta.get(this);
					if (meta && meta.url) {
						const reqInfo = inspectTranscodeRequest(meta.url, meta.headers, w);
						if (reqInfo.isVideoSegment) {
							try {
								Object.defineProperty(this, 'readyState', { value: 4, configurable: true });
								Object.defineProperty(this, 'status', { value: 204, configurable: true });
								Object.defineProperty(this, 'response', { value: new ArrayBuffer(0), configurable: true });
								Object.defineProperty(this, 'responseText', { value: '', configurable: true });
							} catch (_) {}
							const self = this;
							const tid = setTimeout(() => {
								activeTimers.delete(tid);
								try {
									const Ev = w.Event || Event;
									if (typeof self.onreadystatechange === 'function') self.onreadystatechange(new Ev('readystatechange'));
									self.dispatchEvent(new Ev('readystatechange'));
									if (typeof self.onload === 'function') self.onload(new Ev('load'));
									self.dispatchEvent(new Ev('load'));
									if (typeof self.onloadend === 'function') self.onloadend(new Ev('loadend'));
									self.dispatchEvent(new Ev('loadend'));
								} catch (_) {}
							}, 0);
							activeTimers.add(tid);
							return;
						}
					}
					return origSend.apply(this, arguments);
				};
				restorers.push(() => {
					proto.open = origOpen;
					proto.setRequestHeader = origSetHeader;
					proto.send = origSend;
				});
			}
			if (w.MediaSource && w.MediaSource.prototype) {
				const msProto = w.MediaSource.prototype;
				const origAddSB = msProto.addSourceBuffer;
				const origRemoveSB = msProto.removeSourceBuffer;
				msProto.addSourceBuffer = function(mimeType) {
					const mime = String(mimeType || '').toLowerCase();
					activityObserved = true;
					signalWake();
					if (mime.includes('video')) {
						return createVideoSourceBufferShim();
					}
					const sb = origAddSB.apply(this, arguments);
					if (sb) {
						sb.__isRecapAudioSB = true;
						audioSBRef = sb;
						if (typeof sb.addEventListener === 'function') {
							const onUpdateEnd = () => { stepVideoPlayback(); };
							sb.addEventListener('updateend', onUpdateEnd);
							trackedListeners.push({ target: sb, type: 'updateend', fn: onUpdateEnd });
						}
					}
					return sb;
				};
				if (typeof origRemoveSB === 'function') {
					msProto.removeSourceBuffer = function(sb) {
						if (sb && sb.__isRecapVideoShim) return;
						return origRemoveSB.apply(this, arguments);
					};
				}
				restorers.push(() => {
					msProto.addSourceBuffer = origAddSB;
					if (typeof origRemoveSB === 'function') msProto.removeSourceBuffer = origRemoveSB;
				});
			}
			if (w.SourceBuffer && w.SourceBuffer.prototype) {
				const sbProto = w.SourceBuffer.prototype;
				const origAppend = sbProto.appendBuffer;
				sbProto.appendBuffer = function(data) {
					if (this && this.__isRecapVideoShim) return this.appendBuffer(data);
					if (this && this.mime && String(this.mime).toLowerCase().includes('video')) return;
					if (this && !this.__isRecapAudioSB) audioSBRef = this;
					const u8 = toUint8Array(data);
					if (u8) recordMSESegment(u8);
					if (fatalErr) return;
					if (this && this.buffered && this.buffered.length > 0) {
						try {
							const e = this.buffered.end(this.buffered.length - 1);
							if (Number.isFinite(e) && e > mseMaxBufferedEnd) mseMaxBufferedEnd = e;
						} catch (_) {}
					}
					return origAppend.apply(this, arguments);
				};
				restorers.push(() => { sbProto.appendBuffer = origAppend; });
			}
		}
		function cleanupHooks() {
			if (hooksRestored) return;
			hooksRestored = true;
			for (const tid of activeTimers) clearTimeout(tid);
			activeTimers.clear();
			for (const item of trackedListeners) {
				try {
					if (item.target && typeof item.target.removeEventListener === 'function') {
						item.target.removeEventListener(item.type, item.fn);
					}
				} catch (_) {}
			}
			for (let i = restorers.length - 1; i >= 0; i--) {
				try { restorers[i](); } catch (_) {}
			}
			if (abortCtrl) {
				try { abortCtrl.abort(); } catch (_) {}
			}
			const m = playerDoc.querySelector ? playerDoc.querySelector('audio, video') : null;
			if (m && typeof m.pause === 'function') {
				try { m.pause(); } catch (_) {}
			}
		}

		try {
			installWindowHooks(playerWin);
			if (playerWin !== window) installWindowHooks(window);

			let playTriggered = false;
			function triggerPlaybackIfReady() {
				if (playTriggered) return;
				const btn = playerDoc.querySelector ? playerDoc.querySelector(playBtnSelector) : null;
				const m = playerDoc.querySelector ? playerDoc.querySelector('audio, video') : null;
				if (m) {
					try {
						m.muted = true;
						m.volume = 0;
						m.playbackRate = 16;
					} catch (_) {}
				}
				if (btn && typeof btn.click === 'function') {
					playTriggered = true;
					btn.click();
				}
				if (m && typeof m.play === 'function') {
					playTriggered = true;
					try { Promise.resolve(m.play()).catch(() => {}); } catch (_) {}
				}
			}
			triggerPlaybackIfReady();

			const startedAt = Date.now();
			const deadlineAt = startedAt + 120000;
			const initialActivityTimeoutMs = 10000;
			let pathAResult = null;

			while (Date.now() < deadlineAt) {
				if (!playTriggered) triggerPlaybackIfReady();
				if (fatalErr) throw fatalErr;
				maybeStartPathA();
				if (pathAPromise) {
					const settled = await Promise.race([
						pathAPromise.then(res => ({ done: true, res })),
						sleepOrWake(25).then(() => ({ done: false }))
					]);
					if (fatalErr) throw fatalErr;
					if (settled.done && settled.res) {
						pathAResult = settled.res;
						break;
					}
					if (settled.done && !settled.res) {
						pathAPromise = null;
					}
				} else {
					await sleepOrWake(25);
				}
				if (fatalErr) throw fatalErr;
				if (!activityObserved && Date.now() - startedAt >= initialActivityTimeoutMs) {
					break;
				}
				if (!pathAActive) {
					stepVideoPlayback();
					const m = playerDoc.querySelector ? playerDoc.querySelector('audio, video') : null;
					const targetDur = (() => {
						for (const raw of [m && m.duration, info.duration, info.durationSeconds]) {
							const d = Number(raw);
							if (Number.isFinite(d) && d > 0) return d;
						}
						if (transcodePpd > 0) return transcodePpd >= 1e6 ? (transcodePpd / 1e7) : transcodePpd;
						return 0;
					})();
					if (mseInitSegment && mseSegments.size > 0) {
						if (m && m.ended) break;
						if (targetDur > 0 && mseMaxBufferedEnd >= targetDur - 0.5 && Date.now() - mseLastAppendAt >= 150) break;
						if (mseLastAppendAt > 0 && Date.now() - mseLastAppendAt >= 3000) break;
					}
				}
			}
			if (pathAResult && pathAStartedAt > 0) {
				const settleRemaining = 80 - (Date.now() - pathAStartedAt);
				if (settleRemaining > 0) {
					await new Promise(resolve => {
						const tid = setTimeout(() => { activeTimers.delete(tid); resolve(); }, settleRemaining);
						activeTimers.add(tid);
					});
				}
			}
			if (fatalErr) throw fatalErr;

			let initSegment = null;
			let orderedSegments = [];
			let selectedMediaUrl = '';
			if (pathAResult) {
				initSegment = pathAResult.initSegment;
				orderedSegments = pathAResult.mediaSegments;
				selectedMediaUrl = pathAResult.selectedMediaUrl;
			} else {
				initSegment = mseInitSegment;
				orderedSegments = Array.from(mseSegments.values()).sort((a, b) => {
					if (a.dts !== null && b.dts !== null && a.dts !== b.dts) return a.dts - b.dts;
					if (a.dts !== null && b.dts === null) return -1;
					if (a.dts === null && b.dts !== null) return 1;
					return a.seq - b.seq;
				}).map(item => item.data);
				selectedMediaUrl = transcodeAudioTemplateUrl || (candidates.length ? candidates[0] : pageUrl);
			}

			if (!initSegment || !orderedSegments.length) {
				throw (blockedErr || lastFetchErr || new Error('recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)'));
			}
			const totalSourceBytes = initSegment.byteLength + orderedSegments.reduce((n, s) => n + s.byteLength, 0);
			if (totalSourceBytes > maxRecapAudioSourceBytes) {
				throw new Error(limitError);
			}
			const combined = new Uint8Array(totalSourceBytes);
			combined.set(initSegment, 0);
			let off = initSegment.byteLength;
			for (const seg of orderedSegments) {
				combined.set(seg, off);
				off += seg.byteLength;
			}
			const AudioCtxCtor = getAudioCtxCtor();
			if (!AudioCtxCtor) {
				throw new Error('recap audio AudioContext unavailable');
			}
			const audioContext = new AudioCtxCtor({sampleRate: 44100});
			try {
				const pcmState = { totalDurationSec: 0, totalPCMBytes: 0, pcmChunks: [] };
				try {
					const buf = await audioContext.decodeAudioData(combined.buffer);
					decodeAudioBufferToPCMChunks(buf, pcmState);
				} catch (singleErr) {
					const singleMsg = String((singleErr && singleErr.message) || singleErr || '');
					if (singleMsg.includes(limitError)) throw singleErr;
					if (orderedSegments.length <= 1) throw singleErr;
					pcmState.totalDurationSec = 0;
					pcmState.totalPCMBytes = 0;
					pcmState.pcmChunks = [];
					for (const seg of orderedSegments) {
						const zeroed = zeroFMP4DecodeTime(seg);
						const segBuf = new Uint8Array(initSegment.byteLength + zeroed.byteLength);
						segBuf.set(initSegment, 0);
						segBuf.set(zeroed, initSegment.byteLength);
						const buf = await audioContext.decodeAudioData(segBuf.buffer);
						decodeAudioBufferToPCMChunks(buf, pcmState);
					}
				}
				if (pcmState.totalPCMBytes > 4*1024*1024) {
					window.__goTeamsRecapAudioTransfer = { chunks: pcmState.pcmChunks };
					return JSON.stringify({
						pageUrl,
						itemId,
						fileName,
						mediaUrl: sanitizeMediaUrl(selectedMediaUrl),
						sourceBytes: totalSourceBytes,
						durationSec: pcmState.totalDurationSec,
						transferPending: true,
						chunkCount: pcmState.pcmChunks.length
					});
				}
				return JSON.stringify({
					pageUrl,
					itemId,
					fileName,
					mediaUrl: sanitizeMediaUrl(selectedMediaUrl),
					sourceBytes: totalSourceBytes,
					durationSec: pcmState.totalDurationSec,
					pcmChunks: pcmState.pcmChunks
				});
			} finally {
				await audioContext.close();
			}
		} finally {
			cleanupHooks();
		}
	}

	if (streamSurface) {
		try {
			return await captureStreamEmbedAudio();
		} catch (streamErr) {
			const msg = String((streamErr && streamErr.message) || streamErr || '');
			if (msg.includes(limitError) || msg.includes('identity mismatch') || msg.includes('identity unverified')) {
				throw streamErr;
			}
			if (blockedErr) throw blockedErr;
			if (info.isDownloadBlocked) {
				throw new Error('recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)');
			}
			throw (streamErr || lastFetchErr || new Error('recap audio fetch failed'));
		}
	}
	throw (blockedErr || lastFetchErr || new Error('recap audio fetch failed'));
})()`

const maxCDPDOMMessageBytes = 16 * 1024 * 1024

var (
	maxRecapAudioSourceBytes = 256 * 1024 * 1024
	maxRecapAudioDurationSec = 4200
	maxRecapAudioPCMBytes    = 360 * 1024 * 1024
)

func captureRecapAudioFromTargets(ctx context.Context, port int, ownerID, meetingURL string, w io.Writer) error {
	if port <= 0 || ownerID == "" || w == nil {
		return fmt.Errorf("browser: recap audio requires a bound CDP page and writer")
	}
	expected := ExtractMeetingIdentity(meetingURL)
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		return fmt.Errorf("browser: encode recap audio identity: %w", err)
	}
	// Wrap the constants in a function scope so repeated Runtime.evaluate calls
	// in the same execution context do not redeclare global lexical bindings.
	expr := fmt.Sprintf("(() => {\nconst maxRecapAudioSourceBytes=%d, maxRecapAudioDurationSec=%d, maxRecapAudioPCMBytes=%d, recapExpectedIdentity=%s;\nreturn %s;\n})()",
		maxRecapAudioSourceBytes, maxRecapAudioDurationSec, maxRecapAudioPCMBytes, expectedJSON, extractRecapAudioJS)

	var lastErr error
	errPriority := func(err error) int {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "exceeds maximum supported size/duration"):
			return 3
		case strings.Contains(msg, "identity mismatch"), strings.Contains(msg, "identity unverified"):
			return 2
		case strings.Contains(msg, "isDownloadBlocked=true"):
			return 1
		default:
			return 0
		}
	}
	setLastErr := func(err error) {
		if err == nil {
			return
		}
		if lastErr != nil && errPriority(lastErr) > errPriority(err) {
			return
		}
		lastErr = err
	}

	const maxAttempts = 8
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("browser: recap audio cancelled: %w", err)
		}
		targets, listErr := listCDPTargets(port)
		if listErr != nil {
			if attempt == 0 {
				return fmt.Errorf("browser: list recap audio targets: %w", listErr)
			}
			break
		}
		var owner *cdpTargetInfo
		for i := range targets {
			if targets[i].Type == "page" && targets[i].ID == ownerID && targets[i].WebSocketDebuggerURL != "" {
				owner = &targets[i]
				break
			}
		}
		if owner == nil {
			return fmt.Errorf("browser: recap audio page is unavailable")
		}
		if hasMeetingIdentityConflict(expected, ExtractMeetingIdentity(owner.URL)) {
			return fmt.Errorf("browser: recap audio page does not match requested recording")
		}
		frames := expandOwnedFrames(targets, ownerID, fetchOwnedFrames(ctx, owner.WebSocketDebuggerURL))
		orderedTargets := make([]cdpTargetInfo, len(targets))
		copy(orderedTargets, targets)
		hasStreamEmbedTarget := false
		for _, t := range orderedTargets {
			if isTargetInCaptureScope(t, ownerID, frames) && strings.Contains(strings.ToLower(t.URL), "streamembed.aspx") {
				hasStreamEmbedTarget = true
				break
			}
		}
		recapTargetRank := func(t cdpTargetInfo) int {
			lowerURL := strings.ToLower(t.URL)
			if t.Type == "iframe" {
				if strings.Contains(lowerURL, "streamembed.aspx") {
					return 0
				}
				if strings.Contains(lowerURL, "xplatplugins.aspx") {
					return 1
				}
				return 2
			}
			return 3
		}
		sort.SliceStable(orderedTargets, func(i, j int) bool {
			return recapTargetRank(orderedTargets[i]) < recapTargetRank(orderedTargets[j])
		})
		for _, target := range orderedTargets {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("browser: recap audio cancelled: %w", err)
			}
			if isAuthRedirectURL(target.URL) {
				continue
			}
			if !isTargetInCaptureScope(target, ownerID, frames) ||
				hasMeetingIdentityConflict(expected, ExtractMeetingIdentity(target.URL)) {
				continue
			}
			tStart := time.Now()
			raw, evalErr := evalInCDPTargetWithin(ctx, target.WebSocketDebuggerURL, expr, recapAudioEvalTimeout)
			slog.Debug("browser: recap audio eval finished", "elapsed", time.Since(tStart).Round(time.Millisecond), "raw_len", len(raw), "err", evalErr)
			if evalErr != nil {
				setLastErr(evalErr)
				continue
			}
			if raw == "" {
				continue
			}
			var payload struct {
				PageURL         string   `json:"pageUrl"`
				ItemID          string   `json:"itemId"`
				FileName        string   `json:"fileName"`
				MediaURL        string   `json:"mediaUrl"`
				SourceBytes     int64    `json:"sourceBytes"`
				DurationSec     float64  `json:"durationSec"`
				PCMChunks       []string `json:"pcmChunks"`
				TransferPending bool     `json:"transferPending"`
				ChunkCount      int      `json:"chunkCount"`
				Error           string   `json:"error"`
			}
			if err := json.Unmarshal([]byte(raw), &payload); err != nil {
				setLastErr(fmt.Errorf("browser: invalid recap audio payload: %w", err))
				continue
			}
			slog.Debug("browser: recap audio unmarshaled", "elapsed", time.Since(tStart).Round(time.Millisecond), "chunks", len(payload.PCMChunks), "transfer_pending", payload.TransferPending, "chunk_count", payload.ChunkCount, "duration_sec", payload.DurationSec)
			pageIdentity := recapAudioIdentity(payload.PageURL)
			mediaIdentity := recapAudioIdentity(payload.MediaURL)
			if hasMeetingIdentityConflict(expected, pageIdentity) ||
				hasMeetingIdentityConflict(expected, mediaIdentity) ||
				(expected.DriveItemID != "" && payload.ItemID != "" && !strings.EqualFold(expected.DriveItemID, payload.ItemID)) ||
				(expected.RecordingName != "" && payload.FileName != "" && !strings.EqualFold(expected.RecordingName, payload.FileName)) {
				setLastErr(fmt.Errorf("browser: recap audio recording identity mismatch"))
				continue
			}
			if expected != (MeetingIdentity{}) &&
				!hasRecapAudioIdentityMatch(expected, pageIdentity) &&
				!hasRecapAudioIdentityMatch(expected, mediaIdentity) &&
				!(expected.DriveItemID != "" && strings.EqualFold(expected.DriveItemID, payload.ItemID)) &&
				!(expected.RecordingName != "" && strings.EqualFold(expected.RecordingName, payload.FileName)) {
				setLastErr(fmt.Errorf("browser: recap audio recording identity unverified"))
				continue
			}
			if payload.Error != "" {
				setLastErr(fmt.Errorf("browser: recap audio extraction: %s", payload.Error))
				continue
			}
			if payload.SourceBytes < 0 || payload.SourceBytes > int64(maxRecapAudioSourceBytes) ||
				math.IsNaN(payload.DurationSec) || payload.DurationSec < 0 || payload.DurationSec > float64(maxRecapAudioDurationSec) {
				return fmt.Errorf("browser: recap audio exceeds maximum supported size/duration")
			}
			if payload.TransferPending {
				if payload.ChunkCount <= 0 {
					continue
				}
				cleanupTransfer := func() {
					cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cleanCancel()
					_, _ = evalInCDPTarget(cleanCtx, target.WebSocketDebuggerURL, `(() => { delete window.__goTeamsRecapAudioTransfer; return ""; })()`)
				}
				enc := audio.NewEncoder(w, audio.SampleRate)
				var size int64
				for pulled := 0; pulled < payload.ChunkCount; {
					if err := ctx.Err(); err != nil {
						cleanupTransfer()
						return fmt.Errorf("browser: recap audio cancelled: %w", err)
					}
					batchRaw, pullErr := evalInCDPTargetWithin(ctx, target.WebSocketDebuggerURL, `(() => {
						const t = window.__goTeamsRecapAudioTransfer;
						if (!t || !Array.isArray(t.chunks)) return "[]";
						return JSON.stringify(t.chunks.splice(0, 32));
					})()`, 30*time.Second)
					if pullErr != nil {
						cleanupTransfer()
						return fmt.Errorf("browser: pull recap audio chunk batch: %w", pullErr)
					}
					var batch []string
					if err := json.Unmarshal([]byte(batchRaw), &batch); err != nil {
						cleanupTransfer()
						return fmt.Errorf("browser: decode recap audio chunk batch: %w", err)
					}
					if len(batch) == 0 {
						break
					}
					for _, chunk := range batch {
						n := int64(base64.StdEncoding.DecodedLen(len(chunk)))
						if strings.HasSuffix(chunk, "==") {
							n -= 2
						} else if strings.HasSuffix(chunk, "=") {
							n--
						}
						size += n
						if size > int64(maxRecapAudioPCMBytes) {
							cleanupTransfer()
							return fmt.Errorf("browser: recap audio exceeds maximum supported size/duration")
						}
						samples, decErr := decodeAudioPayload(chunk)
						if decErr != nil {
							cleanupTransfer()
							return fmt.Errorf("browser: recap audio: %w", decErr)
						}
						if writeErr := enc.WritePCM(samples); writeErr != nil {
							cleanupTransfer()
							return fmt.Errorf("browser: encode recap audio: %w", writeErr)
						}
					}
					pulled += len(batch)
				}
				cleanupTransfer()
				if size == 0 {
					continue
				}
				if err := enc.Flush(); err != nil {
					return fmt.Errorf("browser: flush recap audio: %w", err)
				}
				slog.Info("browser: captured recap audio", "elapsed", time.Since(tStart).Round(time.Millisecond), "duration_sec", payload.DurationSec, "source_bytes", payload.SourceBytes, "pcm_bytes", size)
				return nil
			}
			var size int64
			for _, chunk := range payload.PCMChunks {
				n := int64(base64.StdEncoding.DecodedLen(len(chunk)))
				if strings.HasSuffix(chunk, "==") {
					n -= 2
				} else if strings.HasSuffix(chunk, "=") {
					n--
				}
				size += n
				if size > int64(maxRecapAudioPCMBytes) {
					return fmt.Errorf("browser: recap audio exceeds maximum supported size/duration")
				}
			}
			if len(payload.PCMChunks) == 0 || size == 0 {
				continue
			}
			enc := audio.NewEncoder(w, audio.SampleRate)
			for _, chunk := range payload.PCMChunks {
				samples, err := decodeAudioPayload(chunk)
				if err != nil {
					return fmt.Errorf("browser: recap audio: %w", err)
				}
				if err := enc.WritePCM(samples); err != nil {
					return fmt.Errorf("browser: encode recap audio: %w", err)
				}
			}
			if err := enc.Flush(); err != nil {
				return fmt.Errorf("browser: flush recap audio: %w", err)
			}
			slog.Info("browser: captured recap audio", "elapsed", time.Since(tStart).Round(time.Millisecond), "duration_sec", payload.DurationSec, "source_bytes", payload.SourceBytes, "pcm_bytes", size)
			return nil
		}
		if lastErr == nil || !strings.Contains(lastErr.Error(), "isDownloadBlocked=true") || hasStreamEmbedTarget || attempt == maxAttempts-1 {
			break
		}
		timer := time.NewTimer(1 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("browser: recap audio cancelled: %w", ctx.Err())
		case <-timer.C:
		}
	}
	if lastErr != nil {
		return fmt.Errorf("browser: recap audio unavailable: %w", lastErr)
	}
	return fmt.Errorf("browser: recap audio unavailable for requested recording")
}

var (
	isoDurationRe  = regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`)
	colonTimeRe    = regexp.MustCompile(`^(?:(\d+):)?(\d{1,2}):(\d{2})$`)
	meetingTokenRe = regexp.MustCompile(`(?i)meeting_[A-Za-z0-9_-]+`)
	meetSegmentRe  = regexp.MustCompile(`(?i)/meet/([A-Za-z0-9_-]+)`)
)

// MeetingIdentity holds the unique meeting and recording identifiers parsed
// from a Teams meeting or Meeting Recap URL so extraction can verify that a
// browser tab or transcript iframe belongs to the intended meeting.
type MeetingIdentity struct {
	ThreadID      string
	MeetingToken  string
	DriveItemID   string
	DriveID       string
	CallID        string
	ICalUID       string
	RecordingName string
	MeetID        string
}

// ExtractMeetingIdentity parses unique meeting identifiers (threadId,
// meeting_<token>, driveItemId, driveId, callId, iCalUid, fileUrl recording
// filename, and /meet/<id>) from rawURL, checking both top-level query
// parameters and SPA hash-fragment query parameters.
func ExtractMeetingIdentity(rawURL string) MeetingIdentity {
	var id MeetingIdentity
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return id
	}
	u, err := url.Parse(trimmed)
	if err == nil {
		applyQueryIdentity(&id, u.Query())
		if u.Fragment != "" {
			if idx := strings.Index(u.Fragment, "?"); idx >= 0 {
				if fragVals, fErr := url.ParseQuery(u.Fragment[idx+1:]); fErr == nil {
					applyQueryIdentity(&id, fragVals)
				}
			}
		}
	}
	decoded := trimmed
	if unescaped, uErr := url.QueryUnescape(trimmed); uErr == nil {
		decoded = unescaped
	}
	if id.MeetingToken == "" {
		if tok := meetingTokenRe.FindString(decoded); tok != "" {
			id.MeetingToken = strings.ToLower(tok)
		}
	}
	if id.MeetID == "" {
		if m := meetSegmentRe.FindStringSubmatch(decoded); len(m) == 2 {
			id.MeetID = strings.ToLower(m[1])
		}
	}
	return id
}

func applyQueryIdentity(id *MeetingIdentity, vals url.Values) {
	if v := strings.TrimSpace(vals.Get("threadId")); v != "" && id.ThreadID == "" {
		id.ThreadID = strings.ToLower(v)
		if tok := meetingTokenRe.FindString(v); tok != "" && id.MeetingToken == "" {
			id.MeetingToken = strings.ToLower(tok)
		}
	}
	if v := strings.TrimSpace(vals.Get("driveItemId")); v != "" && id.DriveItemID == "" {
		id.DriveItemID = strings.ToLower(v)
	}
	if v := strings.TrimSpace(vals.Get("driveId")); v != "" && id.DriveID == "" {
		id.DriveID = strings.ToLower(v)
	}
	if v := strings.TrimSpace(vals.Get("callId")); v != "" && id.CallID == "" {
		id.CallID = strings.ToLower(v)
	}
	if v := strings.TrimSpace(vals.Get("iCalUid")); v != "" && id.ICalUID == "" {
		id.ICalUID = strings.ToLower(v)
	}
	if v := strings.TrimSpace(vals.Get("fileUrl")); v != "" && id.RecordingName == "" {
		if fu, err := url.Parse(v); err == nil && fu.Path != "" && !strings.Contains(fu.Path, "/:") {
			base := filepath.Base(filepath.FromSlash(fu.Path))
			ext := strings.ToLower(filepath.Ext(base))
			if base != "." && base != "/" && (ext == ".mp4" || ext == ".mkv" || ext == ".webm" || ext == ".mov" || ext == ".m4a") {
				id.RecordingName = strings.ToLower(base)
			}
		}
	}
}

// hasPositiveMeetingIdentityMatch reports whether candidateURL carries at least
// one unique meeting identifier that matches expectedURL and no identifiers
// that conflict with expectedURL.
func hasPositiveMeetingIdentityMatch(expectedURL, candidateURL string) bool {
	exp := ExtractMeetingIdentity(expectedURL)
	act := ExtractMeetingIdentity(candidateURL)
	if hasMeetingIdentityConflict(exp, act) {
		return false
	}
	return hasPositiveMeetingIdentityOverlap(exp, act)
}

func hasPositiveMeetingIdentityOverlap(exp, act MeetingIdentity) bool {
	return (exp.ThreadID != "" && exp.ThreadID == act.ThreadID) ||
		(exp.MeetingToken != "" && exp.MeetingToken == act.MeetingToken) ||
		(exp.DriveItemID != "" && exp.DriveItemID == act.DriveItemID) ||
		(exp.CallID != "" && exp.CallID == act.CallID) ||
		(exp.ICalUID != "" && exp.ICalUID == act.ICalUID) ||
		(exp.MeetID != "" && exp.MeetID == act.MeetID)
}

// recapAudioIdentity extends ExtractMeetingIdentity with a recording filename
// taken from a direct media pathname, matching the in-page identity check that
// accepts both a fileUrl parameter and a media URL path as a recording name.
func recapAudioIdentity(rawURL string) MeetingIdentity {
	id := ExtractMeetingIdentity(rawURL)
	if id.RecordingName != "" {
		return id
	}
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Path == "" || strings.Contains(u.Path, "/:") {
		return id
	}
	base := path.Base(u.Path)
	switch strings.ToLower(path.Ext(base)) {
	case ".mp4", ".mkv", ".webm", ".mov", ".m4a":
		id.RecordingName = strings.ToLower(base)
	}
	return id
}

// hasRecapAudioIdentityMatch reports whether act positively matches exp
// without conflict, additionally accepting a matching recording filename.
func hasRecapAudioIdentityMatch(exp, act MeetingIdentity) bool {
	if hasMeetingIdentityConflict(exp, act) {
		return false
	}
	return hasPositiveMeetingIdentityOverlap(exp, act) ||
		(exp.RecordingName != "" && exp.RecordingName == act.RecordingName)
}

func hasMeetingIdentityConflict(exp, act MeetingIdentity) bool {
	if exp.ThreadID != "" && act.ThreadID != "" && exp.ThreadID != act.ThreadID {
		return true
	}
	if exp.MeetingToken != "" && act.MeetingToken != "" && exp.MeetingToken != act.MeetingToken {
		return true
	}
	if exp.DriveItemID != "" && act.DriveItemID != "" && exp.DriveItemID != act.DriveItemID {
		return true
	}
	if exp.DriveID != "" && act.DriveID != "" && exp.DriveID != act.DriveID {
		return true
	}
	if exp.CallID != "" && act.CallID != "" && exp.CallID != act.CallID {
		return true
	}
	if exp.ICalUID != "" && act.ICalUID != "" && exp.ICalUID != act.ICalUID {
		return true
	}
	if exp.RecordingName != "" && act.RecordingName != "" && exp.RecordingName != act.RecordingName {
		return true
	}
	if exp.MeetID != "" && act.MeetID != "" && exp.MeetID != act.MeetID {
		return true
	}
	return false
}

// RecapTranscriptEntry is a single utterance extracted from a Teams Meeting
// Recap transcript panel.
type RecapTranscriptEntry struct {
	Speaker string
	Offset  string
	Text    string
}

// AISummaryProvider is an optional interface implemented by Meeting values that
// capture the Teams Meeting Recap AI Summary alongside the transcript.
type AISummaryProvider interface {
	AISummary() string
}

// MeetingMetadata holds header metadata discovered from the Teams Meeting Recap
// page and transcript.
type MeetingMetadata struct {
	Title     string
	StartTime time.Time
	Timezone  string
	Attendees []string
}

// MetadataProvider is an optional interface implemented by Meeting values that
// supply discovered meeting header metadata.
type MetadataProvider interface {
	MeetingMetadata() MeetingMetadata
}

// RecapMeeting is an optional interface implemented by Meeting values that
// report whether the joined session is a Meeting Recap export rather than a
// live call.
type RecapMeeting interface {
	IsRecap() bool
}

// copyAISummaryJS navigates to the Notes -> AI summary tab on the Teams Meeting
// Recap page, hooks clipboard writes (navigator.clipboard and document copy
// events), clicks the "Copy all" button on the AI summary, and returns the
// copied plain-text summary.
const copyAISummaryJS = `(async () => {
	const notesTab = document.querySelector('button[role="tab"][data-tid="Notes"]');
	if (notesTab && notesTab.getAttribute('aria-selected') !== 'true') {
		notesTab.click();
		await new Promise(r => setTimeout(r, 300));
	}
	const aiNotesTab = document.querySelector('button[role="tab"][data-tid="AINotes"]');
	if (aiNotesTab && aiNotesTab.getAttribute('aria-selected') !== 'true') {
		aiNotesTab.click();
		await new Promise(r => setTimeout(r, 300));
	}
	const copyBtn = document.querySelector('button[aria-label="Copy all"]');
	if (!copyBtn) {
		return '';
	}
	let copiedText = '';
	const copyListener = (e) => {
		if (e.clipboardData) {
			const origSetData = e.clipboardData.setData.bind(e.clipboardData);
			e.clipboardData.setData = (type, val) => {
				if (type === 'text/plain') copiedText = String(val || '');
				return origSetData(type, val);
			};
		}
	};
	document.addEventListener('copy', copyListener, true);
	if (navigator.clipboard) {
		const origWriteText = navigator.clipboard.writeText ? navigator.clipboard.writeText.bind(navigator.clipboard) : null;
		navigator.clipboard.writeText = async (txt) => {
			copiedText = String(txt || '');
			if (origWriteText) {
				try { await origWriteText(txt); } catch (_) {}
			}
		};
		const origWrite = navigator.clipboard.write ? navigator.clipboard.write.bind(navigator.clipboard) : null;
		navigator.clipboard.write = async (items) => {
			try {
				for (const item of items) {
					if (item.types && item.types.includes('text/plain')) {
						const blob = await item.getType('text/plain');
						copiedText = await blob.text();
					}
				}
			} catch (_) {}
			if (origWrite) {
				try { await origWrite(items); } catch (_) {}
			}
		};
	}
	copyBtn.click();
	await new Promise(r => setTimeout(r, 500));
	document.removeEventListener('copy', copyListener, true);
	return copiedText;
})()`

// ownedFrameSet holds the verified frame IDs belonging to a specific page
// target's frame tree (obtained via Page.getFrameTree).
type ownedFrameSet struct {
	ids map[string]bool
}

func (s ownedFrameSet) contains(t cdpTargetInfo) bool {
	return t.ID != "" && s.ids[t.ID]
}

func expandOwnedFrames(targets []cdpTargetInfo, capturePageID string, base ownedFrameSet) ownedFrameSet {
	out := ownedFrameSet{
		ids: make(map[string]bool, len(base.ids)),
	}
	for id, ok := range base.ids {
		if ok && id != "" {
			out.ids[id] = true
		}
	}
	if capturePageID == "" {
		return out
	}
	for {
		added := false
		for _, t := range targets {
			if t.ID == "" || out.ids[t.ID] || t.ParentID == "" {
				continue
			}
			if t.ParentID == capturePageID || out.ids[t.ParentID] {
				out.ids[t.ID] = true
				added = true
			}
		}
		if !added {
			break
		}
	}
	return out
}

// isTargetInCaptureScope reports whether t belongs to the active capture page
// (either the page tab itself or one of its child iframes in ownedFrames) so
// unrelated open or restored browser tabs are never scraped or attached.
func isTargetInCaptureScope(t cdpTargetInfo, capturePageID string, ownedFrames ownedFrameSet) bool {
	if (t.Type != "page" && t.Type != "iframe") || t.WebSocketDebuggerURL == "" {
		return false
	}
	if strings.HasPrefix(t.URL, "edge://") || strings.HasPrefix(t.URL, "chrome-extension://") {
		return false
	}
	if capturePageID == "" {
		return true
	}
	if t.Type == "page" {
		return t.ID == capturePageID
	}
	// Real Chromium/Edge /json/list populates parentId on out-of-process
	// iframes (OOPIFs such as cross-origin SharePoint xplatplugins.aspx), while
	// Page.getFrameTree only lists same-process local frames. Check parentId
	// against capturePageID and ownedFrames first, then fall back to frame IDs.
	if t.ParentID != "" {
		return t.ParentID == capturePageID || ownedFrames.ids[t.ParentID] || ownedFrames.contains(t)
	}
	return ownedFrames.contains(t)
}

type cdpFrameTreeNode struct {
	Frame struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	} `json:"frame"`
	ChildFrames []cdpFrameTreeNode `json:"childFrames"`
}

func collectFrameTree(node cdpFrameTreeNode, out *ownedFrameSet) {
	if node.Frame.ID != "" {
		out.ids[node.Frame.ID] = true
	}
	for _, child := range node.ChildFrames {
		collectFrameTree(child, out)
	}
}

func fetchOwnedFrames(ctx context.Context, pageWSURL string) ownedFrameSet {
	out := ownedFrameSet{
		ids: make(map[string]bool),
	}
	if pageWSURL == "" {
		return out
	}
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, _, _, err := ws.Dial(dialCtx, pageWSURL)
	if err != nil {
		return out
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	req := map[string]any{
		"id":     1,
		"method": "Page.getFrameTree",
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return out
	}
	if err := wsutil.WriteClientText(conn, payload); err != nil {
		return out
	}
	for {
		msg, _, err := wsutil.ReadServerData(conn)
		if err != nil {
			return out
		}
		var resp struct {
			ID     int `json:"id"`
			Result struct {
				FrameTree cdpFrameTreeNode `json:"frameTree"`
			} `json:"result"`
		}
		if err := json.Unmarshal(msg, &resp); err == nil && resp.ID == 1 {
			collectFrameTree(resp.Result.FrameTree, &out)
			return out
		}
	}
}

func createCDPPageTarget(ctx context.Context, browserWSURL, initialURL string) (string, error) {
	if initialURL == "" {
		initialURL = "about:blank"
	}
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, _, _, err := ws.Dial(dialCtx, browserWSURL)
	if err != nil {
		return "", fmt.Errorf("browser: dial browser websocket: %w", err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	req := map[string]any{
		"id":     1,
		"method": "Target.createTarget",
		"params": map[string]any{
			"url": initialURL,
		},
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	if err := wsutil.WriteClientText(conn, payload); err != nil {
		return "", err
	}
	for {
		msg, _, err := wsutil.ReadServerData(conn)
		if err != nil {
			return "", err
		}
		var resp struct {
			ID    int `json:"id"`
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
			Result struct {
				TargetID string `json:"targetId"`
			} `json:"result"`
		}
		if err := json.Unmarshal(msg, &resp); err == nil && resp.ID == 1 {
			if resp.Error != nil {
				return "", fmt.Errorf("browser: Target.createTarget error %d: %s", resp.Error.Code, resp.Error.Message)
			}
			if resp.Result.TargetID == "" {
				return "", fmt.Errorf("browser: Target.createTarget returned empty targetId")
			}
			return resp.Result.TargetID, nil
		}
	}
}

func captureAISummaryFromTargets(ctx context.Context, port int, ownerPageID string) string {
	for attempt := 0; attempt < 20; attempt++ {
		if ctx.Err() != nil {
			return ""
		}
		// The outer recap page and its Copy all button can appear after the
		// transcript iframe, so discover targets again on every attempt.
		targets, err := listCDPTargets(port)
		if err == nil {
			for _, t := range targets {
				if t.Type != "page" || t.WebSocketDebuggerURL == "" {
					continue
				}
				if strings.HasPrefix(t.URL, "edge://") || strings.HasPrefix(t.URL, "chrome-extension://") {
					continue
				}
				if ownerPageID != "" && t.ID != ownerPageID {
					continue
				}
				if isAuthRedirectURL(t.URL) {
					return ""
				}
				if raw, err := evalInCDPTarget(ctx, t.WebSocketDebuggerURL, copyAISummaryJS); err == nil && strings.TrimSpace(raw) != "" {
					return strings.TrimSpace(raw)
				}
			}
		}
		if attempt == 19 {
			break
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ""
		case <-timer.C:
		}
	}
	return ""
}

type cdpTargetInfo struct {
	ID                   string `json:"id"`
	ParentID             string `json:"parentId"`
	Type                 string `json:"type"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// cleanBrowserArgs returns the minimal, untampered command-line flags used to
// launch a browser instance (headed or --headless=new) without stealth flags
// such as --disable-blink-features=AutomationControlled. The Chromium sandbox
// stays enabled unless the TEAMS_DISABLE_SANDBOX opt-in is set.
func cleanBrowserArgs(port int, profileDir string, headless bool, initialURL string) []string {
	return cleanBrowserArgsForOptions(port, profileDir, Options{Headless: headless}, initialURL)
}

// cleanBrowserArgsForOptions builds the clean direct-exec browser arguments
// for opts. The Chromium sandbox stays enabled by default; --no-sandbox and
// --disable-setuid-sandbox are appended, before the trailing initialURL, only
// when shouldDisableSandbox(opts) opts in.
func cleanBrowserArgsForOptions(port int, profileDir string, opts Options, initialURL string) []string {
	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--no-first-run",
		"--no-default-browser-check",
		"--use-fake-ui-for-media-stream",
		"--auto-select-desktop-capture-source=Microsoft Teams",
	}
	if opts.Headless {
		args = append(args, "--headless=new")
	}
	if profileDir != "" {
		args = append(args, fmt.Sprintf("--user-data-dir=%s", profileDir))
	}
	if shouldDisableSandbox(opts) {
		args = append(args, "--no-sandbox", "--disable-setuid-sandbox")
	}
	if initialURL != "" {
		args = append(args, initialURL)
	}
	return args
}

// hasHTTPSHostSuffix parses rawURL and reports whether it is an https:// URL
// whose hostname equals one of domains or is a dot-prefixed subdomain of one
// of domains.
func hasHTTPSHostSuffix(rawURL string, domains ...string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || !strings.EqualFold(u.Scheme, "https") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}
	for _, d := range domains {
		d = strings.ToLower(d)
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// isAuthRedirectURL reports whether rawURL is on a Microsoft / identity
// provider login host where CDP Runtime.evaluate probes should be paused until
// interactive sign-in redirects back to Teams.
func isAuthRedirectURL(rawURL string) bool {
	return hasHTTPSHostSuffix(rawURL,
		"login.microsoftonline.com",
		"login.microsoft.com",
		"login.live.com",
		"account.live.com",
		"login.windows.net",
		"login.microsoftonline.us",
		"autologon.microsoftazuread-sso.com",
		"device.login.microsoftonline.com",
		"account.activedirectory.windowsazure.com",
		"mysignins.microsoft.com",
		"fpt.microsoft.com",
		"aadcdn.msftauth.net",
		"msftauth.net",
		"msauth.net",
		"duosecurity.com",
		"okta.com",
		"pingidentity.com",
		"onelogin.com",
		"auth0.com",
	)
}

func isTeamsHostURL(rawURL string) bool {
	return hasHTTPSHostSuffix(rawURL,
		"teams.cloud.microsoft",
		"teams.microsoft.com",
		"teams.live.com",
	)
}

func isMicrosoftTeamsRecapURL(rawURL string) bool {
	return isTeamsHostURL(rawURL) && strings.Contains(strings.ToLower(rawURL), "meetingrecap")
}

var profileEnvVarRe = regexp.MustCompile(`%([A-Za-z_][A-Za-z0-9_]*)%|\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

func expandProfilePath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	expanded := profileEnvVarRe.ReplaceAllStringFunc(trimmed, func(match string) string {
		sub := profileEnvVarRe.FindStringSubmatch(match)
		name := ""
		for i := 1; i < len(sub); i++ {
			if sub[i] != "" {
				name = sub[i]
				break
			}
		}
		if name == "" {
			return match
		}
		if v, ok := os.LookupEnv(name); ok && v != "" {
			return v
		}
		if strings.HasPrefix(match, "%") && (strings.EqualFold(name, "USER") || strings.EqualFold(name, "USERPROFILE")) {
			if home, err := os.UserHomeDir(); err == nil && home != "" {
				return home
			}
		}
		return match
	})
	if expanded == "~" || strings.HasPrefix(expanded, "~/") || strings.HasPrefix(expanded, `~\`) {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			if expanded == "~" {
				expanded = home
			} else {
				expanded = filepath.Join(home, expanded[2:])
			}
		}
	}
	if runtime.GOOS != "windows" {
		expanded = strings.ReplaceAll(expanded, `\`, "/")
	}
	return filepath.Clean(expanded)
}

func isRepoRootDir(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return true
	}
	return false
}

func replaceRightmostPathSegment(p, oldSeg, newSeg string) string {
	vol := filepath.VolumeName(p)
	rest := strings.TrimPrefix(p, vol)
	hasLeadingSep := len(rest) > 0 && os.IsPathSeparator(rest[0])
	parts := strings.FieldsFunc(rest, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	replaced := false
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.EqualFold(parts[i], oldSeg) {
			prefixJoined := strings.Join(parts[:i+1], string(filepath.Separator))
			var candidateRoot string
			if hasLeadingSep {
				candidateRoot = filepath.Clean(vol + string(filepath.Separator) + prefixJoined)
			} else if vol != "" {
				candidateRoot = filepath.Clean(vol + prefixJoined)
			} else {
				candidateRoot = filepath.Clean(prefixJoined)
			}
			if isRepoRootDir(candidateRoot) {
				continue
			}
			parts[i] = newSeg
			replaced = true
			break
		}
	}
	if !replaced {
		return p
	}
	joined := strings.Join(parts, string(filepath.Separator))
	if hasLeadingSep {
		return filepath.Clean(vol + string(filepath.Separator) + joined)
	}
	if vol != "" {
		return filepath.Clean(vol + joined)
	}
	return filepath.Clean(joined)
}

func resolveExistingProfileDir(raw string) string {
	return resolveExpandedProfileDir(expandProfilePath(raw))
}

func resolveExpandedProfileDir(expanded string) string {
	if expanded == "" {
		return ""
	}
	if hasTeamsSessionInExpandedProfile(expanded) {
		return expanded
	}
	legacy := replaceRightmostPathSegment(expanded, "go-teams", "teams-cli")
	if legacy != expanded && hasTeamsSessionInExpandedProfile(legacy) {
		return legacy
	}
	return expanded
}

// hasTeamsSessionInProfile reports whether profileDir already contains a
// persisted Microsoft Teams web session in Chromium's Default Local Storage.
// When false on a cold or newly configured profile directory, headed recap
// capture bootstraps authentication at https://teams.cloud.microsoft/v2/ first
// to avoid Teams' cold-profile /l/meetingrecap OAuth redirect loop.
func hasTeamsSessionInProfile(profileDir string) bool {
	return hasTeamsSessionInExpandedProfile(expandProfilePath(profileDir))
}

func hasTeamsSessionInExpandedProfile(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return false
	}
	leveldbDir := filepath.Join(abs, "Default", "Local Storage", "leveldb")
	entries, err := os.ReadDir(leveldbDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if !strings.HasSuffix(name, ".ldb") && !strings.HasSuffix(name, ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() <= 0 || info.Size() > 16*1024*1024 {
			continue
		}
		data, err := os.ReadFile(filepath.Join(leveldbDir, e.Name()))
		if err != nil {
			continue
		}
		if bytes.Contains(data, []byte("SKYPE-TOKEN")) ||
			bytes.Contains(data, []byte("UserAuthState")) ||
			bytes.Contains(data, []byte("msal.2.account.keys")) {
			return true
		}
	}
	return false
}

func cleanHeadedBrowserArgs(port int, profileDir, initialURL string) []string {
	return cleanBrowserArgs(port, profileDir, false, initialURL)
}

// ParseRecapTimestamp parses an ISO-8601 duration (e.g. "PT15M29S", "PT1H2M3S")
// or colon-separated offset (e.g. "3:07", "01:12:45") into a time.Time anchored
// at midnight on base's date so Format("15:04:05") renders the exact HH:MM:SS
// recording offset.
func ParseRecapTimestamp(raw string, base time.Time) (time.Time, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, false
	}
	var hours, minutes, seconds int
	if m := isoDurationRe.FindStringSubmatch(trimmed); m != nil {
		if m[1] == "" && m[2] == "" && m[3] == "" {
			return time.Time{}, false
		}
		if m[1] != "" {
			hours, _ = strconv.Atoi(m[1])
		}
		if m[2] != "" {
			minutes, _ = strconv.Atoi(m[2])
		}
		if m[3] != "" {
			seconds, _ = strconv.Atoi(m[3])
		}
	} else if m := colonTimeRe.FindStringSubmatch(trimmed); m != nil {
		if m[1] != "" {
			hours, _ = strconv.Atoi(m[1])
		}
		minutes, _ = strconv.Atoi(m[2])
		seconds, _ = strconv.Atoi(m[3])
	} else {
		return time.Time{}, false
	}
	midnight := time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location())
	d := time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
	return midnight.Add(d), true
}

const extractRecapTranscriptJS = `(() => {
	const fi = window.g_fileInfo || {};
	const fileTitle = fi.title ? String(fi.title).trim() : '';
	const fileName = fi.name ? String(fi.name).trim() : '';
	const itemId = String(fi.VroomItemID || fi.VroomItemId || fi.itemId || fi.driveItemId || fi.id || fi.docId || '').trim();
	const pageUrl = String(location.href || '');
	const fileStart = String(fi.CreatedDateTime || fi.TimeCreated || fi.creationTime || fi.lastModifiedTime || fi.name || '').trim();
	const tz = (typeof Intl !== 'undefined' && Intl.DateTimeFormat) ? (Intl.DateTimeFormat().resolvedOptions().timeZone || '') : '';
	const listEl = document.querySelector('.ms-List');
	if (listEl) {
		const fiberKey = Object.keys(listEl).find(k => k.startsWith('__reactFiber$') || k.startsWith('__reactInternalInstance$'));
		let cur = fiberKey ? listEl[fiberKey] : null;
		for (let depth = 0; cur && depth < 25; depth++, cur = cur.return) {
			const p = cur.memoizedProps || cur.pendingProps;
			if (p && Array.isArray(p.items) && p.items.length > 0) {
				const entries = p.items.map(it => ({
					speaker: String(it.speakerDisplayName || it.speaker || '').trim(),
					timestamp: String(it.timestamp || it.startTime || '').trim(),
					text: String(it.text || '').trim(),
					type: String(it.type || 'Text')
				}));
				return JSON.stringify({ title: fileTitle, fileName: fileName, itemId: itemId, pageUrl: pageUrl, startTime: fileStart, timezone: tz, entries });
			}
		}
	}
	const cells = Array.from(document.querySelectorAll('.ms-List-cell, [data-transcript-entry-id]'));
	if (cells.length === 0) {
		return JSON.stringify({ title: fileTitle, fileName: fileName, itemId: itemId, pageUrl: pageUrl, startTime: fileStart, timezone: tz, entries: [] });
	}
	const entries = [];
	const seen = new Set();
	for (const cell of cells) {
		const idEl = cell.hasAttribute('data-transcript-entry-id') ? cell : cell.querySelector('[data-transcript-entry-id]');
		const id = idEl ? idEl.getAttribute('data-transcript-entry-id') : '';
		if (id && seen.has(id)) continue;
		if (id) seen.add(id);
		const spkEl = cell.querySelector('[data-transcript-copy-field="speaker"], [class*="itemDisplayName-"]');
		const tsEl = cell.querySelector('[data-transcript-copy-field="timestamp"], [id^="Header-timestamp-"]');
		const txtEl = cell.querySelector('[class*="entryText-"]');
		if (!txtEl) continue;
		const text = (txtEl.textContent || '').trim();
		if (!text) continue;
		entries.push({
			speaker: spkEl ? (spkEl.textContent || '').trim() : '',
			timestamp: tsEl ? (tsEl.textContent || '').trim() : '',
			text: text,
			type: 'Text'
		});
	}
	return JSON.stringify({ title: fileTitle, fileName: fileName, itemId: itemId, pageUrl: pageUrl, startTime: fileStart, timezone: tz, entries });
})()`

const extractRecapPageMetadataJS = `(() => {
	const rawTitle = (document.title || '').replace(/^\(\d+\)\s*/, '').replace(/\s*\[[^\]]+\]\s*$/, '').replace(/\s*\|\s*Microsoft Teams.*$/i, '').trim();
	const headerEl = document.querySelector('[data-tid="intelligent-recap-header"]');
	const headerText = headerEl ? (headerEl.parentElement ? headerEl.parentElement.innerText : headerEl.innerText) : '';
	const bodyText = (headerText + '\n' + (document.body ? document.body.innerText : '')).replace(/[\u202f\u00a0]/g, ' ');
	const m = bodyText.match(/(?:[A-Za-z]+,\s+)?([A-Za-z]+\s+\d{1,2}(?:,\s*\d{4})?)[\s,•·|\-–—]+(\d{1,2}:\d{2}\s*[AP]M)/i);
	const startText = m ? (m[1] + ' ' + m[2]) : '';
	const tz = (typeof Intl !== 'undefined' && Intl.DateTimeFormat) ? (Intl.DateTimeFormat().resolvedOptions().timeZone || '') : '';
	return JSON.stringify({ title: rawTitle, startText: startText, timezone: tz });
})()`

type rawRecapPayload struct {
	Title     string `json:"title"`
	FileName  string `json:"fileName"`
	ItemID    string `json:"itemId"`
	PageURL   string `json:"pageUrl"`
	StartTime string `json:"startTime"`
	Timezone  string `json:"timezone"`
	Entries   []struct {
		Speaker   string `json:"speaker"`
		Timestamp string `json:"timestamp"`
		Text      string `json:"text"`
		Type      string `json:"type"`
	} `json:"entries"`
}

func verifyRecapTargetMatchesMeeting(expectedMeetingURL, pageURL, targetURL string, p rawRecapPayload) bool {
	exp := ExtractMeetingIdentity(expectedMeetingURL)
	for _, observedURL := range []string{pageURL, targetURL, p.PageURL} {
		if observedURL == "" {
			continue
		}
		if hasMeetingIdentityConflict(exp, ExtractMeetingIdentity(observedURL)) {
			return false
		}
	}
	if exp.RecordingName != "" && p.FileName != "" && !strings.EqualFold(exp.RecordingName, strings.TrimSpace(p.FileName)) {
		return false
	}
	if exp.DriveItemID != "" && p.ItemID != "" && strings.HasPrefix(strings.ToLower(strings.TrimSpace(p.ItemID)), "01") && !strings.Contains(strings.ToLower(p.ItemID), exp.DriveItemID) {
		return false
	}
	return true
}

func parseRecapEntriesPayload(raw string) ([]RecapTranscriptEntry, error) {
	entries, _, err := parseRecapPayloadWithMeta(raw)
	return entries, err
}

func parseRecapPayloadForMeeting(raw, expectedMeetingURL, pageURL, targetURL string) ([]RecapTranscriptEntry, MeetingMetadata, error) {
	var p rawRecapPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, MeetingMetadata{}, fmt.Errorf("browser: parse recap transcript payload: %w", err)
	}
	if !verifyRecapTargetMatchesMeeting(expectedMeetingURL, pageURL, targetURL, p) {
		return nil, MeetingMetadata{}, fmt.Errorf("browser: recap target does not match requested meeting identity")
	}
	return buildRecapEntriesAndMeta(p)
}

func parseRecapPayloadWithMeta(raw string) ([]RecapTranscriptEntry, MeetingMetadata, error) {
	var p rawRecapPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, MeetingMetadata{}, fmt.Errorf("browser: parse recap transcript payload: %w", err)
	}
	return buildRecapEntriesAndMeta(p)
}

func buildRecapEntriesAndMeta(p rawRecapPayload) ([]RecapTranscriptEntry, MeetingMetadata, error) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	out := make([]RecapTranscriptEntry, 0, len(p.Entries))
	speakerSet := make(map[string]struct{})
	for _, item := range p.Entries {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		speaker := strings.TrimSpace(item.Speaker)
		if speaker == "" {
			speaker = "Speaker"
		} else {
			speakerSet[speaker] = struct{}{}
		}
		var offset string
		if ts, ok := ParseRecapTimestamp(item.Timestamp, base); ok {
			offset = ts.Format("15:04:05")
		}
		out = append(out, RecapTranscriptEntry{
			Speaker: speaker,
			Offset:  offset,
			Text:    text,
		})
	}
	attendees := make([]string, 0, len(speakerSet))
	for spk := range speakerSet {
		attendees = append(attendees, spk)
	}
	sort.Strings(attendees)
	meta := MeetingMetadata{
		Title:     strings.TrimSpace(p.Title),
		Timezone:  strings.TrimSpace(p.Timezone),
		Attendees: attendees,
	}
	if ts, ok := parseRecapStartTimeInLocation(p.StartTime, meta.Timezone); ok {
		meta.StartTime = ts
	}
	return out, meta, nil
}

func enrichRecapMetadataFromPage(ctx context.Context, port int, ownerPageID string, meta *MeetingMetadata) {
	targets, err := listCDPTargets(port)
	if err != nil {
		return
	}
	for _, t := range targets {
		if t.Type != "page" || t.WebSocketDebuggerURL == "" {
			continue
		}
		if ownerPageID != "" && t.ID != ownerPageID {
			continue
		}
		raw, err := evalInCDPTarget(ctx, t.WebSocketDebuggerURL, extractRecapPageMetadataJS)
		if err != nil || raw == "" {
			continue
		}
		var pageMeta struct {
			Title     string `json:"title"`
			StartText string `json:"startText"`
			Timezone  string `json:"timezone"`
		}
		if json.Unmarshal([]byte(raw), &pageMeta) != nil {
			continue
		}
		if meta.Title == "" && pageMeta.Title != "" && !strings.EqualFold(pageMeta.Title, "Microsoft Teams") {
			meta.Title = pageMeta.Title
		}
		if meta.Timezone == "" && pageMeta.Timezone != "" {
			meta.Timezone = pageMeta.Timezone
		}
		if meta.StartTime.IsZero() && pageMeta.StartText != "" {
			if parsed, ok := parseRecapStartTimeInLocation(pageMeta.StartText, meta.Timezone); ok {
				meta.StartTime = parsed
			}
		}
		return
	}
}

var (
	ampmNoSpaceRe   = regexp.MustCompile(`(?i)(\d{1,2}:\d{2})\s*([ap]m)$`)
	recordingNameRe = regexp.MustCompile(`(\d{8})_(\d{6})`)
)

func parseRecapStartTime(raw string) (time.Time, bool) {
	return parseRecapStartTimeInLocation(raw, "")
}

func parseRecapStartTimeInLocation(raw, tz string) (time.Time, bool) {
	cleaned := strings.NewReplacer("\u202f", " ", "\u00a0", " ", ",", ", ").Replace(raw)
	normalized := strings.Join(strings.Fields(strings.TrimSpace(cleaned)), " ")
	normalized = strings.ReplaceAll(normalized, " ,", ",")
	normalized = ampmNoSpaceRe.ReplaceAllStringFunc(normalized, func(m string) string {
		sub := ampmNoSpaceRe.FindStringSubmatch(m)
		return sub[1] + " " + strings.ToUpper(sub[2])
	})
	if t, err := time.Parse(time.RFC3339, normalized); err == nil {
		loc := time.Local
		if tz != "" {
			if l, locErr := time.LoadLocation(tz); locErr == nil {
				loc = l
			}
		}
		return t.In(loc).Round(time.Minute), true
	}
	for _, layout := range []string{
		"Monday, January 2, 2006 3:04 PM",
		"Monday, January 02, 2006 3:04 PM",
		"January 2, 2006 3:04 PM",
		"January 02, 2006 3:04 PM",
		"Jan 2, 2006 3:04 PM",
		"Jan 02, 2006 3:04 PM",
	} {
		if t, err := time.Parse(layout, normalized); err == nil {
			return t, true
		}
	}
	for _, layout := range []string{
		"January 2 3:04 PM",
		"January 02 3:04 PM",
		"Jan 2 3:04 PM",
		"Jan 02 3:04 PM",
	} {
		if t, err := time.Parse(layout, normalized); err == nil {
			nowYear := time.Now().Year()
			return time.Date(nowYear, t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC), true
		}
	}
	if m := recordingNameRe.FindStringSubmatch(normalized); m != nil {
		if t, err := time.Parse("20060102_150405", m[1]+"_"+m[2]); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func launchCleanHeadedBrowser(opts Options, initialURL string, profileResolved bool) (int, string, string, <-chan struct{}, func(), error) {
	execPath := ResolveExecPath(opts.ExecPath)
	if execPath == "" {
		return 0, "", "", nil, nil, fmt.Errorf("browser: no Chromium-based browser executable found")
	}
	var absProfile string
	if opts.UserDataDir != "" {
		resolvedProfile := opts.UserDataDir
		if !profileResolved {
			resolvedProfile = resolveExistingProfileDir(opts.UserDataDir)
		}
		abs, err := filepath.Abs(resolvedProfile)
		if err != nil {
			return 0, "", "", nil, nil, fmt.Errorf("browser: resolve profile dir: %w", err)
		}
		if err := os.MkdirAll(abs, 0o700); err != nil {
			return 0, "", "", nil, nil, fmt.Errorf("browser: create profile dir: %w", err)
		}
		absProfile = abs
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, "", "", nil, nil, fmt.Errorf("browser: allocate debugging port: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	// Launch directly with initialURL so the browser opens a single tab (no
	// extra about:blank tab). Once DevTools is ready, bind capturePageID to
	// that single page target (or create one if none exists yet).
	targetURL := initialURL
	if targetURL == "" {
		targetURL = "about:blank"
	}
	args := cleanBrowserArgsForOptions(port, absProfile, opts, targetURL)
	cmd := exec.Command(execPath, args...)
	if err := cmd.Start(); err != nil {
		return 0, "", "", nil, nil, fmt.Errorf("browser: start browser: %w", err)
	}
	procDone := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(procDone)
	}()
	var wsURL string
	cleanup := func() {
		if wsURL != "" {
			closeCDPBrowser(wsURL)
		}
		if cmd.Process != nil {
			select {
			case <-procDone:
			default:
				_ = cmd.Process.Kill() // Best-effort handle-based termination when launcher has not yet exited.
				<-procDone
			}
		}
		if runtime.GOOS == "windows" {
			terminateSurvivingCDPBrowserOnPort(port, execPath)
		}
		deadline := time.Now().Add(2 * time.Second)
		teardownClean := false
		for time.Now().Before(deadline) {
			if _, probeErr := listCDPTargets(port); probeErr != nil {
				teardownClean = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if !teardownClean {
			slog.Warn("browser: CDP port still reachable after browser cleanup", "cdp_port", port)
		}
	}

	wsURL, err = waitForCDPBrowserWS(port, 20*time.Second)
	if err != nil {
		cleanup()
		return 0, "", "", nil, nil, err
	}
	targetID := waitForInitialPageTargetID(port, 5*time.Second)
	if targetID == "" {
		targetID, err = createCDPPageTarget(context.Background(), wsURL, targetURL)
		if err != nil {
			cleanup()
			return 0, "", "", nil, nil, err
		}
	}
	slog.Info("browser: launched clean browser", "pid", cmd.Process.Pid, "cdp_port", port, "target_id", targetID, "headless", opts.Headless)
	return port, wsURL, targetID, procDone, cleanup, nil
}

func waitForInitialPageTargetID(port int, timeout time.Duration) string {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		targets, err := listCDPTargets(port)
		if err == nil {
			var candidates []cdpTargetInfo
			for _, t := range targets {
				if t.Type != "page" || t.WebSocketDebuggerURL == "" {
					continue
				}
				if strings.HasPrefix(t.URL, "edge://") || strings.HasPrefix(t.URL, "chrome-extension://") {
					continue
				}
				candidates = append(candidates, t)
			}
			if len(candidates) == 1 {
				return candidates[0].ID
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return ""
}

func waitForCDPBrowserWS(port int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	versionURL := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(versionURL)
		if err == nil {
			var v struct {
				WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&v)
			_ = resp.Body.Close()
			if v.WebSocketDebuggerURL != "" {
				return v.WebSocketDebuggerURL, nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return "", fmt.Errorf("browser: DevTools endpoint on port %d did not become ready", port)
}

func listCDPTargets(port int) ([]cdpTargetInfo, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var targets []cdpTargetInfo
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return nil, err
	}
	return targets, nil
}

// cdpEvalTimeout bounds short DOM probes evaluated over CDP.
var cdpEvalTimeout = 10 * time.Second

// recapAudioEvalTimeout bounds Recap audio extraction, which downloads,
// decodes, and serializes a whole recording before returning. The caller's
// context still cancels the evaluation earlier.
var recapAudioEvalTimeout = 5 * time.Minute

func evalInCDPTarget(ctx context.Context, wsURL, expr string) (string, error) {
	audioLimit := int64(maxRecapAudioPCMBytes)*2 + 65536
	limit := min(int64(maxCDPDOMMessageBytes), audioLimit)
	return evalInCDPTargetWithLimit(ctx, wsURL, expr, cdpEvalTimeout, limit)
}

// evalInCDPTargetWithin evaluates expr in the CDP target and waits up to
// timeout for the result. Cancelling ctx closes the connection so a blocked
// read returns promptly.
func evalInCDPTargetWithin(ctx context.Context, wsURL, expr string, timeout time.Duration) (string, error) {
	return evalInCDPTargetWithLimit(ctx, wsURL, expr, timeout, int64(maxRecapAudioPCMBytes)*2+65536)
}

func evalInCDPTargetWithLimit(ctx context.Context, wsURL, expr string, timeout time.Duration, limit int64) (string, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, _, _, err := ws.Dial(dialCtx, wsURL)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() }) // unblocks the pending read on cancellation
	defer stop()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	req := map[string]any{
		"id":     1,
		"method": "Runtime.evaluate",
		"params": map[string]any{
			"expression":    expr,
			"returnByValue": true,
			"awaitPromise":  true,
		},
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	if err := wsutil.WriteClientText(conn, payload); err != nil {
		return "", err
	}
	for {
		msg, err := readBoundedCDPMessage(conn, limit)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return "", ctxErr
			}
			return "", err
		}
		var resp struct {
			ID    int `json:"id"`
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
			Result struct {
				Result struct {
					Type  string `json:"type"`
					Value any    `json:"value"`
				} `json:"result"`
				ExceptionDetails any `json:"exceptionDetails"`
			} `json:"result"`
		}
		if err := json.Unmarshal(msg, &resp); err == nil && resp.ID == 1 {
			if resp.Error != nil {
				return "", fmt.Errorf("browser: cdp error %d: %s", resp.Error.Code, resp.Error.Message)
			}
			if resp.Result.ExceptionDetails != nil {
				eb, _ := json.Marshal(resp.Result.ExceptionDetails)
				return "", fmt.Errorf("browser: js exception: %s", string(eb))
			}
			if resp.Result.Result.Value == nil {
				return "", nil
			}
			if s, ok := resp.Result.Result.Value.(string); ok {
				return s, nil
			}
			vb, _ := json.Marshal(resp.Result.Result.Value)
			return string(vb), nil
		}
	}
}

func readBoundedCDPMessage(conn io.ReadWriter, limit int64) ([]byte, error) {
	var message bytes.Buffer
	started := false
	for {
		hdr, err := ws.ReadHeader(conn)
		if err != nil {
			return nil, err
		}
		if hdr.Length < 0 || hdr.Length > limit || hdr.Length > limit-int64(message.Len()) {
			return nil, fmt.Errorf("browser: cdp response exceeds maximum supported size/duration")
		}
		if hdr.Rsv != 0 {
			return nil, fmt.Errorf("browser: invalid cdp websocket frame")
		}
		switch hdr.OpCode {
		case ws.OpText, ws.OpBinary:
			if started {
				return nil, fmt.Errorf("browser: unexpected cdp websocket data frame")
			}
			started = true
		case ws.OpContinuation:
			if !started {
				return nil, fmt.Errorf("browser: unexpected cdp websocket continuation")
			}
		case ws.OpPing, ws.OpPong, ws.OpClose:
			if !hdr.Fin || hdr.Length > 125 {
				return nil, fmt.Errorf("browser: invalid cdp websocket control frame")
			}
		default:
			return nil, fmt.Errorf("browser: unsupported cdp websocket opcode")
		}
		frame := make([]byte, int(hdr.Length))
		if _, err := io.ReadFull(conn, frame); err != nil {
			return nil, err
		}
		if hdr.Masked {
			ws.Cipher(frame, hdr.Mask, 0)
		}
		switch hdr.OpCode {
		case ws.OpPing:
			if err := wsutil.WriteClientMessage(conn, ws.OpPong, frame); err != nil {
				return nil, err
			}
			continue
		case ws.OpPong:
			continue
		case ws.OpClose:
			return nil, io.EOF
		}
		if _, err := message.Write(frame); err != nil {
			return nil, err
		}
		if hdr.Fin {
			return message.Bytes(), nil
		}
	}
}

func closeCDPBrowser(wsURL string) {
	if strings.TrimSpace(wsURL) == "" {
		return
	}
	dialCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, _, _, err := ws.Dial(dialCtx, wsURL)
	if err != nil {
		return
	}
	defer func() {
		_ = conn.Close() // Best-effort CDP websocket close during teardown.
	}()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second)) // Best-effort deadline on teardown socket.
	payload, err := json.Marshal(map[string]any{
		"id":     1,
		"method": "Browser.close",
	})
	if err != nil {
		return
	}
	if err := wsutil.WriteClientText(conn, payload); err != nil {
		return
	}
	_, _, _ = wsutil.ReadServerData(conn) // Best-effort wait for Browser.close ack; browser may close socket immediately.
}

func isCDPPortStillAlive(port int) bool {
	if port <= 0 {
		return false
	}
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err != nil {
		return false
	}
	defer func() {
		_ = resp.Body.Close() // Best-effort close of local DevTools version probe response body.
	}()
	var v struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return false
	}
	return v.WebSocketDebuggerURL != ""
}

func parseNetstatListeningPID(output string, port int) int {
	if port <= 0 {
		return 0
	}
	targetAddr := fmt.Sprintf("127.0.0.1:%d", port)
	for _, rawLine := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(rawLine))
		if len(fields) < 5 {
			continue
		}
		if !strings.EqualFold(fields[0], "TCP") || fields[1] != targetAddr || !strings.EqualFold(fields[3], "LISTENING") {
			continue
		}
		pid, err := strconv.Atoi(fields[4])
		if err == nil && pid > 4 {
			return pid
		}
	}
	return 0
}

func findListeningPIDOnLoopbackPort(port int) int {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "netstat", "-ano", "-p", "tcp").Output()
	if err != nil {
		return 0
	}
	return parseNetstatListeningPID(string(out), port)
}

func isChromiumProcessImage(pid int, expectedExecPath string) bool {
	if pid <= 4 {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH").Output()
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(out))
	for _, img := range []string{"msedge.exe", "chrome.exe", "chromium.exe", "brave.exe"} {
		if strings.Contains(lower, img) {
			return true
		}
	}
	if base := strings.ToLower(strings.TrimSpace(filepath.Base(expectedExecPath))); base != "" && base != "." {
		if strings.Contains(lower, base) {
			return true
		}
	}
	return false
}

func terminateSurvivingCDPBrowserOnPort(port int, execPath string) {
	time.Sleep(100 * time.Millisecond)
	if !isCDPPortStillAlive(port) {
		return
	}
	pid := findListeningPIDOnLoopbackPort(port)
	if pid <= 4 || !isChromiumProcessImage(pid, execPath) {
		return
	}
	killCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = exec.CommandContext(killCtx, "taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run() // Best-effort termination of verified surviving Chromium child process on the ephemeral CDP port.
}

// navigateCDPTarget sends a top-level CDP Page.navigate command to wsURL,
// which is the programmatic equivalent of pasting targetURL into the browser's
// address bar. Unlike Runtime.evaluate("location.href = ..."), Page.navigate
// succeeds even when the tab is mid-redirect in an authentication loop.
func navigateCDPTarget(ctx context.Context, wsURL, targetURL string) error {
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, _, _, err := ws.Dial(dialCtx, wsURL)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	req := map[string]any{
		"id":     1,
		"method": "Page.navigate",
		"params": map[string]any{
			"url": targetURL,
		},
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if err := wsutil.WriteClientText(conn, payload); err != nil {
		return err
	}
	for {
		msg, _, err := wsutil.ReadServerData(conn)
		if err != nil {
			return err
		}
		var resp struct {
			ID    int `json:"id"`
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(msg, &resp); err == nil && resp.ID == 1 {
			if resp.Error != nil {
				return fmt.Errorf("browser: Page.navigate error %d: %s", resp.Error.Code, resp.Error.Message)
			}
			return nil
		}
	}
}
