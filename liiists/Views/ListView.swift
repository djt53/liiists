import SwiftUI

struct ListView: View {
    @EnvironmentObject private var store: ListStore
    @Environment(\.colorScheme) private var colorScheme
    @Environment(\.dismiss) private var dismiss
    @State private var list: ItemList
    @State private var newItemText = ""
    @State private var showRename = false
    @State private var renameText = ""
    @State private var showDeleteConfirm = false
    @State private var searchText = ""
    @State private var showSearch = false
    @State private var isAdding = false
    @FocusState private var isAddFieldFocused: Bool
    @FocusState private var isSearchFocused: Bool

    var isNewList: Bool = false

    init(list: ItemList, focusAddField: Bool = false) {
        _list = State(initialValue: list)
        self.isNewList = focusAddField
    }

    private var filteredItems: [ListItem] {
        guard !searchText.isEmpty else { return list.items }
        let query = searchText.lowercased()
        return list.items.filter { $0.text.lowercased().contains(query) }
    }

    /// Whether to show the add-item field
    private var addFieldVisible: Bool {
        list.items.isEmpty || isAdding
    }

    var body: some View {
        VStack(spacing: 0) {
            // Title + search/plus icons
            HStack(alignment: .center) {
                Text(list.title)
                    .font(Theme.headingFont(size: Theme.headingSize, weight: .medium))
                    .foregroundStyle(Theme.ndTextDisplay.resolve(for: colorScheme))
                    .tracking(-0.01 * Theme.headingSize)
                Spacer()
                Button {
                    withAnimation(.easeOut(duration: 0.2)) {
                        showSearch.toggle()
                        if showSearch {
                            isSearchFocused = true
                        } else {
                            searchText = ""
                        }
                    }
                } label: {
                    Image(systemName: "magnifyingglass")
                        .font(.system(size: 16, weight: .light))
                        .foregroundStyle(Theme.ndTextPrimary.resolve(for: colorScheme))
                        .frame(width: 36, height: 36)
                }
                Button {
                    startAdding()
                } label: {
                    Image(systemName: "plus")
                        .font(.system(size: 16, weight: .light))
                        .foregroundStyle(Theme.ndTextPrimary.resolve(for: colorScheme))
                        .frame(width: 36, height: 36)
                }
            }
            .padding(.horizontal, Theme.spaceMD)
            .padding(.top, Theme.spaceLG)
            .padding(.bottom, showSearch ? Theme.spaceSM : Theme.spaceMD)

            // Expandable search bar
            if showSearch {
                HStack(spacing: Theme.spaceSM) {
                    Image(systemName: "magnifyingglass")
                        .font(.system(size: 14, weight: .light))
                        .foregroundStyle(Theme.ndTextSecondary.resolve(for: colorScheme))
                    TextField("Search\u{2026}", text: $searchText)
                        .font(Theme.bodyFont())
                        .foregroundStyle(Theme.ndTextPrimary.resolve(for: colorScheme))
                        .focused($isSearchFocused)
                    Button {
                        searchText = ""
                        showSearch = false
                    } label: {
                        Image(systemName: "xmark.circle.fill")
                            .font(.system(size: 14))
                            .foregroundStyle(Theme.ndTextSecondary.resolve(for: colorScheme))
                    }
                }
                .padding(.horizontal, Theme.spaceMD)
                .padding(.vertical, Theme.spaceSM)
                .background(Theme.ndSurfaceRaised.resolve(for: colorScheme))
                .clipShape(RoundedRectangle(cornerRadius: Theme.cornerRadius))
                .padding(.horizontal, Theme.spaceMD)
                .padding(.bottom, Theme.spaceSM)
            }

            // Add item field
            if addFieldVisible {
                HStack(spacing: Theme.spaceSM) {
                    if list.type == .checklist {
                        Circle()
                            .strokeBorder(Theme.ndBorderVisible.resolve(for: colorScheme).opacity(0.5), lineWidth: Theme.checkboxStroke)
                            .frame(width: Theme.checkboxSize, height: Theme.checkboxSize)
                    }
                    TextField("Add item\u{2026}", text: $newItemText)
                        .font(Theme.bodyFont())
                        .foregroundStyle(Theme.ndTextPrimary.resolve(for: colorScheme))
                        .focused($isAddFieldFocused)
                        .submitLabel(.next)
                        .onSubmit {
                            addItem()
                        }
                }
                .padding(.horizontal, Theme.spaceMD)
                .padding(.bottom, Theme.spaceSM)
                .overlay(alignment: .bottom) {
                    Rectangle()
                        .frame(height: 1)
                        .foregroundStyle(
                            isAddFieldFocused
                                ? Theme.ndTextPrimary.resolve(for: colorScheme)
                                : Theme.ndBorder.resolve(for: colorScheme)
                        )
                        .padding(.horizontal, Theme.spaceMD)
                }
            }

            // Items
            List {
                ForEach(searchText.isEmpty ? $list.items : .constant(filteredItems)) { $item in
                    ItemRow(item: $item, listType: list.type) {
                        Theme.lightHaptic()
                    }
                    .listRowBackground(Color.clear)
                    .listRowInsets(EdgeInsets(top: 0, leading: 0, bottom: 0, trailing: 0))
                    .listRowSeparatorTint(Theme.ndBorder.resolve(for: colorScheme))
                    .swipeActions(edge: .trailing, allowsFullSwipe: true) {
                        Button(role: .destructive) {
                            if let idx = list.items.firstIndex(where: { $0.id == item.id }) {
                                list.items.remove(at: idx)
                                Theme.lightHaptic()
                            }
                        } label: {
                            Image(systemName: "trash")
                        }
                    }
                }
                .onMove { source, destination in
                    list.items.move(fromOffsets: source, toOffset: destination)
                }
            }
            .listStyle(.plain)
            .scrollContentBackground(.hidden)
            .refreshable {
                await Task { @MainActor in
                    let dir = SharedContainer.listsDirectory
                    let url = dir.appendingPathComponent(list.filename)
                    let coordinator = NSFileCoordinator()
                    var error: NSError?
                    coordinator.coordinate(readingItemAt: url, options: [], error: &error) { coordURL in
                        if let content = try? String(contentsOf: coordURL, encoding: .utf8) {
                            let refreshed = MarkdownParser.parse(content: content, filename: list.filename)
                            list.items = refreshed.items
                        }
                    }
                }.value
            }
        }
        .background(Theme.ndBlack.resolve(for: colorScheme))
        .navigationBarTitleDisplayMode(.inline)
        .navigationBarBackButtonHidden(true)
        .toolbar {
            ToolbarItem(placement: .navigationBarLeading) {
                Button {
                    dismiss()
                } label: {
                    Image(systemName: "chevron.left")
                        .font(.system(size: 16, weight: .light))
                        .foregroundStyle(Theme.ndTextPrimary.resolve(for: colorScheme))
                }
            }
            ToolbarItem(placement: .primaryAction) {
                Menu {
                    Button {
                        renameText = list.title
                        showRename = true
                    } label: {
                        Label("Rename List", systemImage: "pencil")
                    }

                    Button {
                        shareListAsText()
                    } label: {
                        Label("Copy as Text", systemImage: "doc.on.doc")
                    }

                    Divider()

                    Button(role: .destructive) {
                        showDeleteConfirm = true
                    } label: {
                        Label("Delete List", systemImage: "trash")
                    }
                } label: {
                    Image(systemName: "ellipsis")
                        .font(.system(size: 16, weight: .light))
                        .foregroundStyle(Theme.ndTextPrimary.resolve(for: colorScheme))
                }
            }
        }
        .alert("Rename List", isPresented: $showRename) {
            TextField("List name", text: $renameText)
            Button("Save") {
                let trimmed = renameText.trimmingCharacters(in: .whitespaces)
                guard !trimmed.isEmpty else { return }
                store.rename(&list, to: trimmed)
            }
            Button("Cancel", role: .cancel) {}
        }
        .alert("Delete List?", isPresented: $showDeleteConfirm) {
            Button("Delete", role: .destructive) {
                store.delete(list)
                dismiss()
            }
            Button("Cancel", role: .cancel) {}
        } message: {
            Text("This will permanently delete \"\(list.title)\" and all its items.")
        }
        .onChange(of: list) { _, newValue in
            store.update(newValue)
        }
        .onChange(of: isAddFieldFocused) { _, focused in
            if !focused && !isSubmitting && newItemText.trimmingCharacters(in: .whitespaces).isEmpty && !list.items.isEmpty {
                isAdding = false
            }
        }
        .onAppear {
            if isNewList || list.items.isEmpty {
                startAdding()
            }
        }
    }

    // MARK: - Add Item Logic

    private func startAdding() {
        isAdding = true
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) {
            isAddFieldFocused = true
        }
    }

    @State private var isSubmitting = false

    private func addItem() {
        let entries = Self.parseEntries(from: newItemText)
        guard !entries.isEmpty else {
            return
        }
        isSubmitting = true
        for entry in entries.reversed() {
            list.items.insert(ListItem(text: entry), at: 0)
        }
        Theme.lightHaptic()
        newItemText = ""
        isAdding = true
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) {
            isAddFieldFocused = true
            isSubmitting = false
        }
    }

    static func parseEntries(from raw: String) -> [String] {
        let lines = raw.split(whereSeparator: { $0.isNewline })
        var results: [String] = []
        for line in lines {
            var trimmed = line.trimmingCharacters(in: .whitespaces)
            if trimmed.isEmpty { continue }
            // Strip common bullet markers
            let bulletPrefixes: [Character] = ["-", "*", "•", "·", "–", "—"]
            if let first = trimmed.first, bulletPrefixes.contains(first) {
                trimmed = String(trimmed.dropFirst()).trimmingCharacters(in: .whitespaces)
            } else {
                // Strip numbered prefixes like "1." "1)" "12:"
                let scanner = Scanner(string: trimmed)
                scanner.charactersToBeSkipped = nil
                if let _ = scanner.scanCharacters(from: .decimalDigits),
                   let sep = scanner.scanCharacter(),
                   sep == "." || sep == ")" || sep == ":" {
                    trimmed = String(trimmed[scanner.currentIndex...]).trimmingCharacters(in: .whitespaces)
                }
            }
            if !trimmed.isEmpty {
                results.append(trimmed)
            }
        }
        if results.isEmpty {
            let fallback = raw.trimmingCharacters(in: .whitespacesAndNewlines)
            if !fallback.isEmpty { return [fallback] }
        }
        return results
    }

    private func shareListAsText() {
        var lines = [list.title]
        for item in list.items {
            if list.type == .checklist {
                let check = item.isChecked ? "[x]" : "[ ]"
                lines.append("\(check) \(item.text)")
            } else {
                lines.append("- \(item.text)")
            }
        }
        UIPasteboard.general.string = lines.joined(separator: "\n")
        Theme.lightHaptic()
    }
}

// MARK: - Item Row

struct ItemRow: View {
    @Binding var item: ListItem
    let listType: ItemList.ListType
    @Environment(\.colorScheme) private var colorScheme
    @FocusState private var isEditing: Bool
    var onToggle: (() -> Void)?

    var body: some View {
        HStack(spacing: Theme.spaceSM) {
            if listType == .checklist {
                Button {
                    withAnimation(Theme.nothingEasing) {
                        item.isChecked.toggle()
                    }
                    onToggle?()
                } label: {
                    checkboxIcon
                }
                .buttonStyle(.plain)
            }

            TextField("", text: $item.text, axis: .vertical)
                .font(Theme.bodyFont())
                .foregroundStyle(
                    listType == .checklist && item.isChecked
                        ? Theme.ndTextDisabled.resolve(for: colorScheme)
                        : Theme.ndTextPrimary.resolve(for: colorScheme)
                )
                .focused($isEditing)
                .lineLimit(nil)

            Spacer()
        }
        .frame(minHeight: Theme.rowMinHeight)
        .padding(.horizontal, Theme.spaceMD)
    }

    @ViewBuilder
    private var checkboxIcon: some View {
        if item.isChecked {
            Image(systemName: "checkmark.circle.fill")
                .font(.system(size: Theme.checkboxSize))
                .foregroundStyle(Theme.ndSuccess)
        } else {
            Circle()
                .strokeBorder(Theme.ndBorderVisible.resolve(for: colorScheme), lineWidth: Theme.checkboxStroke)
                .frame(width: Theme.checkboxSize, height: Theme.checkboxSize)
        }
    }
}
