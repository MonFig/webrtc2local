const cameraSelect = document.getElementById('cameraSelect');
const player = document.getElementById('player');
const timeline = document.getElementById('timeline');

async function loadCameras() {
  const res = await fetch('/api/cameras');
  const data = await res.json();
  cameraSelect.innerHTML = '';
  data.cameras.forEach(c => {
    const opt = document.createElement('option');
    opt.value = c.name;
    opt.textContent = c.name;
    cameraSelect.appendChild(opt);
  });
  if (data.cameras.length > 0) {
    loadTimeline(data.cameras[0].name);
  }
}

async function loadTimeline(camera) {
  const res = await fetch(`/api/timeline/${camera}`);
  const data = await res.json();
  timeline.innerHTML = '';
  const groups = {};
  data.files.forEach(f => {
    if (!groups[f.date]) groups[f.date] = [];
    groups[f.date].push(f);
  });
  Object.keys(groups).sort().reverse().forEach(date => {
    const group = document.createElement('div');
    group.className = 'date-group';
    const title = document.createElement('h3');
    title.textContent = date;
    group.appendChild(title);
    const list = document.createElement('div');
    list.className = 'segment-list';
    groups[date].forEach(f => {
      const btn = document.createElement('div');
      btn.className = 'segment';
      const start = new Date(f.start);
      btn.textContent = start.toLocaleTimeString();
      btn.onclick = () => {
        player.src = f.url;
        player.play();
      };
      list.appendChild(btn);
    });
    group.appendChild(list);
    timeline.appendChild(group);
  });
}

cameraSelect.addEventListener('change', () => {
  loadTimeline(cameraSelect.value);
});

loadCameras();
