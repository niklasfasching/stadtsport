import {Component, h} from "https://niklasfasching.github.io/chocolate.js/src/app.mjs";

/**
search inputs for
venue
event

storage

*/

export class Search extends Component {
  async init() {

  }
  view() {
    return [["nav", "stadtsport"],
            [Form],
            [Results]];
  }
}

class Results extends Component {
  rows = []

  async oncreate() {
    this.position = await getPosition().catch(err => console.log(err));
  }

  async onrender() {
    if (window.location.search === "") return this.results = [];
    if (this.search === window.location.search) return;
    const params = new URLSearchParams(window.location.search);
    const position = this.position,
          event = params.get("event") || "",
          fuzzy = params.get("fuzzy") || "",
          venue = params.get("venue") || "",
          plan = params.get("plan") || "",
          start = params.get("start") || '00:00:00',
          end = params.get("end") || '23:59:00',
          days = (params.get("days") || "").split(",").filter(Boolean),
          distance = params.get("distance") || -1,
          type = params.get("type") || "Either",
          categories = (params.get("categories") || "").split(",").filter(Boolean);
    this.rows = await query(`
      SELECT v.*,
             ${this.position ? `haversine(${this.position.lat}, ${this.position.lon}, v.Lat, v.Lon)` : "0"} AS Distance,
             json_group_array(distinct json_object('Name', e.Name,
                                                   'ID', e.ID,
                                                   'Day', substr('SunMonTueWedThuFriSat', 1 + 3*strftime('%w', e.Date), 3),
                                                   'StartTime', e.StartTime,
                                                   'Category', e.CategoryName,
                                                   'EndTime', e.EndTime,
                                                   'Plans', e.Plans,
                                                   'Type', e.Type)) AS Events
      FROM venues v JOIN events e ON v.ID = e.VenueID
      WHERE e.Name LIKE '%${event}%'
            AND (${!fuzzy} OR
                 e.Name LIKE '%${fuzzy}%' OR
                 v.Name LIKE '%${fuzzy}%' OR
                 e.CategoryName LIKE '%${fuzzy}%')
            AND v.Name LIKE '%${venue}%'
            AND e.Plans LIKE '%${plan}%'
            AND (${distance === -1} OR Distance < ${distance})
            AND (${days.length === 0} OR substr('SunMonTueWedThuFriSat', 1 + 3*strftime('%w', e.Date), 3) IN (${days.map(x => `'${x}'`).join(", ")}))
            AND (${type === "Either"} OR e.Type = '${type.toLowerCase()}')
            AND ((e.Type == 'free' AND e.EndTime >= '${start}') OR (e.Type == 'class' AND e.StartTime >= '${start}' AND e.EndTime <= '${end}'))
            AND (${categories.length === 0} OR e.CategoryName IN (${categories.map(x => `'${x}'`).join(", ")}))
    GROUP BY v.ID
    ORDER BY Distance
    LIMIT 20`);
    this.search = window.location.search;
    app.render();
  }

  view() {
    console.log(this.rows)
    return [".results",
            this.rows.map(row => {
              return [".result",
                      [".venue",
                       ["a.name", {target: "_blank", rel: "noreferrer",
                                   href: `https://urbansportsclub.com/en/venues/${slugify(row.Name)}`}, row.Name],
                       [".district", row.District],
                       [".address", `${row.Address}, ${row.PostalCode} (${Math.round(row.Distance)} km)`]],
                      [".events",
                       groupBy(row.Events, e => `${e.Name}:${e.Type}:${e.Plans}`).map(es => {
                         const {ID, Name, Plans, Type} = es[0];
                         return [".event",
                                 ["a.name", {target: "_blank", rel: "noreferrer", href: `https://urbansportsclub.com/en/class-details/${ID}`}, Name],
                                 [".plan", JSON.parse(Plans)[0]],
                                 [".type", Type],
                                 [".times", es.map(e => [".time", `${e.Day} ${e.StartTime.slice(0, -3)}-${e.EndTime.slice(0, -3)}`])]]
                       })]];
            })];
  }
}

export class Form extends Component {
  plans = ["S", "M", "L", "XL"]
  types = ["Free", "Class", "Either"]
  days = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]
  categories = []

  async oncreate() {
    this.categories = (await query("SELECT distinct CategoryName AS Value FROM events ORDER BY 1 ASC")).map(r => r.Value);
    app.render();
  }

  onPlanChange(e) { app.route(undefined, {plan: e.target.value}); }

  onTypeChange(e) { app.route(undefined, {type: e.target.value}); }

  onDayChange(e) { app.route(undefined, {days: [...e.target.closest("fieldset").querySelectorAll("input")].filter(i => i.checked).map(i => i.value)}); }

  onCategoriesChange(e) { app.route(undefined, {categories: [...e.target.options].filter(o => o.selected).map(o => o.value)}); }

  onStartChange(e) { app.route(undefined, {start: e.target.value}) }
  onEndChange(e) { app.route(undefined, {end: e.target.value}) }

  onDistanceChange(e) { app.route(undefined, {distance: e.target.value}) }

  onInput(e) {
    if (e.key !== "Enter" && e.key !== "Tab") return;
    app.route(undefined, {[e.target.name]: e.target.value});
    e.preventDefault();
  }

  view() {
    const params = new URLSearchParams(window.location.search);

    return ["form",

            ["fieldset.fuzzy",
             ["legend", "Fuzzy"],
             ["input", {type: "text", name: "fuzzy", onkeydown: this.onInput, value: params.get("fuzzy")}]],

            ["fieldset.event",
             ["legend", "Event"],
             ["input", {type: "text", name: "event", onkeydown: this.onInput, value: params.get("event")}]],

            ["fieldset.venue",
             ["legend", "Venue"],
             ["input", {type: "text", name: "venue", onkeydown: this.onInput, value: params.get("venue")}]],

            ["fieldset.plan", {onchange: this.onPlanChange},
             ["legend", "Plan"],
             this.plans.map(p => ["label", ["input", {type: "radio", name: "plan", value: p,
                                                      checked: params.get("plan") === p}], p])],

            ["fieldset.type", {onchange: this.onTypeChange},
             ["legend", "Type"],
             this.types.map(t => ["label", ["input", {type: "radio", name: "type", value: t,
                                                      checked: params.get("type") === t}], t])],

            ["fieldset.day", {onchange: this.onDayChange},
             ["legend", "Day"],
             this.days.map(d => ["label", ["input", {type: "checkbox", value:  d, checked: params.get("days")?.split(",").includes(d)}, d], d])],

            ["fieldset.time",
             ["legend", "Time"],
             ["label", "Start", ["input.start", {type: "time", onchange: this.onStartChange, value: params.get("start") || "00:00:00"}]],
             ["label", "End", ["input.end", {type: "time", onchange: this.onEndChange, value: params.get("end") || "23:59:59"}]]],

            ["fieldset.distance",
             ["legend", `Max Distance (${params.get("distance") || 10} km)`],
             ["input.distance", {type: "range", max: 50, value: params.get("distance") || 10, onchange: this.onDistanceChange }]
            ],

            ["fieldset.categories",
             ["legend", "Cateogries"],
             ["select.categories", {multiple: true, onchange: this.onCategoriesChange},
               this.categories.map(c => ["option", {value: c, selected: params.get("categories")?.split(",").includes(c) ? "" : false}, c])]],

            ["input", {type: "reset", onclick: () => app.route(location.pathname)}],
           ]
  }
}

function slugify(s) {
  return s
    .normalize('NFD').replace(/[\u0300-\u036f]/g, "") // replace umlauts with normal letters
    .replace(/[^\d\w]+/g, "-")
    .toLowerCase();
}


function getPosition() {
  return new Promise((resolve, reject) => {
    if (!navigator.geolocation) return reject(new Error("geo location not supported"));
    navigator.geolocation.getCurrentPosition(p => resolve({lat: p.coords.latitude, lon: p.coords.longitude}),
                                             err => reject(new Error(err.message)));
  });
}

function groupBy(xs, f) {
  const m = xs.reduce(function(m, x) {
    const k = f(x);
    (m[k] = m[k] || []).push(x);
    return m;
  }, {});
  return Object.values(m);
}

function query(q, ...args) {
  let url = `/api/db?query=${encodeURIComponent(q)}`;
  for (const arg of args) url += `&arg=${encodeURIComponent(arg)}`;
  return fetch(url).then(r => r.json()).then(result => {
    if (result.error) throw new Error(result.error);
    return result;
  });
}
