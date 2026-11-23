(() => {
	const prefix = "tau-exam:completed:v1:";
	const dateLabel = new Intl.DateTimeFormat("ru-RU", {dateStyle: "long", timeStyle: "medium"});
	function readAttempt(id) {
		try {
			const a = JSON.parse(localStorage.getItem(prefix + id));
			if (a?.ID === id && Number.isFinite(Date.parse(a.FinishedAt)) && Array.isArray(a.Questions)) return a;
		} catch {}
		return null;
	}
	function showHistory(list) {
		const status = document.getElementById("history-status");
		list.replaceChildren();
		try {
			const attempts = Object.keys(localStorage).filter(key => key.startsWith(prefix))
				.map(key => readAttempt(key.slice(prefix.length))).filter(Boolean)
				.sort((a, b) => Date.parse(b.FinishedAt) - Date.parse(a.FinishedAt));
			status.hidden = attempts.length > 0;
			for (const a of attempts) {
				const link = document.createElement("a");
				link.href = "/?attempt=" + encodeURIComponent(a.ID);
				link.textContent = "Экзамен · " + dateLabel.format(new Date(a.FinishedAt));
				const item = document.createElement("li");
				item.append(link);
				list.append(item);
			}
		} catch {
			status.hidden = false;
			status.textContent = "История экзаменов недоступна в этом браузере.";
		}
	}
	async function showArchived(root) {
		const a = readAttempt(root.dataset.id);
		if (!a) {
			root.textContent = "Эта попытка не найдена в истории браузера.";
			return;
		}
		if (root.querySelector(".attempt-questions")) return;
		document.getElementById("archived-date").textContent = dateLabel.format(new Date(a.FinishedAt));
		try {
			const response = await fetch("/archive/render", {
				method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify(a)
			});
			if (!response.ok) throw new Error();
			root.innerHTML = await response.text();
		} catch {
			root.textContent = "Не удалось открыть архивный билет. Обновите страницу, чтобы попробовать снова.";
		}
	}
	function loadArchive() {
		const completed = document.getElementById("completed-attempt");
		if (completed) {
			try {
				const a = JSON.parse(completed.textContent);
				localStorage.setItem(prefix + a.ID, completed.textContent);
			} catch {
				document.getElementById("archive-status").textContent = "Не удалось сохранить экзамен в истории этого браузера.";
			}
		}
		const list = document.getElementById("exam-history");
		if (list) showHistory(list);
		const archived = document.getElementById("archived-exam");
		if (archived) showArchived(archived);
	}
	function fitPager() {
		const nav = document.querySelector(".pagination");
		if (!nav) return;
		nav.classList.remove("compact-icons");
		nav.classList.add("expanded");
		if (nav.scrollWidth > nav.clientWidth + 1) {
			nav.classList.remove("expanded");
			nav.classList.toggle("compact-icons", nav.scrollWidth > nav.clientWidth + 1);
		}
	}
	const diagramSources = new WeakSet();
	let mermaidModule;
	let diagramID = 0;
	async function renderDiagrams() {
		for (const source of document.querySelectorAll(".lecture pre > code.language-mermaid")) {
			if (diagramSources.has(source)) continue;
			diagramSources.add(source);
			try {
				mermaidModule ??= import("https://cdn.jsdelivr.net/npm/mermaid@11.16.0/dist/mermaid.esm.min.mjs")
					.then(({ default: mermaid }) => {
						mermaid.initialize({ startOnLoad: false, securityLevel: "strict", suppressErrorRendering: true });
						return mermaid;
					});
				const mermaid = await mermaidModule;
				await document.fonts.ready;
				if (!source.isConnected) continue;
				const { svg } = await mermaid.render(`lecture-diagram-${++diagramID}`, source.textContent);
				const diagram = document.createElement("figure");
				diagram.className = "lecture-diagram";
				diagram.innerHTML = svg;
				source.parentElement.replaceWith(diagram);
			} catch (error) {
				console.warn("Не удалось отрисовать диаграмму Mermaid.", error);
			}
		}
	}
	function init() {
		loadArchive();
		fitPager();
		renderDiagrams();
		try {
			for (const field of document.querySelectorAll("textarea[data-draft-key]")) {
				const saved = sessionStorage.getItem(field.dataset.draftKey);
				if (saved !== null) field.value = saved;
			}
		} catch {}
	}
	document.addEventListener("input", ({ target }) => {
		if (target.matches("textarea[data-draft-key]")) {
			try { sessionStorage.setItem(target.dataset.draftKey, target.value); } catch {}
		}
	});
	init();
	document.body.addEventListener("htmx:afterSwap", init);
	document.body.addEventListener("htmx:historyRestore", init);
	window.addEventListener("pageshow", event => { if (event.persisted) init(); });
	window.addEventListener("resize", fitPager);
	document.fonts.ready.then(fitPager);
	document.fonts.addEventListener("loadingdone", fitPager);
})();
