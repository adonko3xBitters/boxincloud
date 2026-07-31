// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'database.dart';

// ignore_for_file: type=lint
class $CachedComicsTable extends CachedComics
    with TableInfo<$CachedComicsTable, CachedComic> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $CachedComicsTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _idMeta = const VerificationMeta('id');
  @override
  late final GeneratedColumn<String> id = GeneratedColumn<String>(
    'id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _serverIdMeta = const VerificationMeta(
    'serverId',
  );
  @override
  late final GeneratedColumn<String> serverId = GeneratedColumn<String>(
    'server_id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _libraryIdMeta = const VerificationMeta(
    'libraryId',
  );
  @override
  late final GeneratedColumn<String> libraryId = GeneratedColumn<String>(
    'library_id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _titleMeta = const VerificationMeta('title');
  @override
  late final GeneratedColumn<String> title = GeneratedColumn<String>(
    'title',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _seriesIdMeta = const VerificationMeta(
    'seriesId',
  );
  @override
  late final GeneratedColumn<String> seriesId = GeneratedColumn<String>(
    'series_id',
    aliasedName,
    true,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
  );
  static const VerificationMeta _seriesNameMeta = const VerificationMeta(
    'seriesName',
  );
  @override
  late final GeneratedColumn<String> seriesName = GeneratedColumn<String>(
    'series_name',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
    defaultValue: const Constant(''),
  );
  static const VerificationMeta _numberMeta = const VerificationMeta('number');
  @override
  late final GeneratedColumn<String> number = GeneratedColumn<String>(
    'number',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
    defaultValue: const Constant(''),
  );
  static const VerificationMeta _folderPathMeta = const VerificationMeta(
    'folderPath',
  );
  @override
  late final GeneratedColumn<String> folderPath = GeneratedColumn<String>(
    'folder_path',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
    defaultValue: const Constant(''),
  );
  static const VerificationMeta _pageCountMeta = const VerificationMeta(
    'pageCount',
  );
  @override
  late final GeneratedColumn<int> pageCount = GeneratedColumn<int>(
    'page_count',
    aliasedName,
    false,
    type: DriftSqlType.int,
    requiredDuringInsert: false,
    defaultValue: const Constant(0),
  );
  static const VerificationMeta _coverPathMeta = const VerificationMeta(
    'coverPath',
  );
  @override
  late final GeneratedColumn<String> coverPath = GeneratedColumn<String>(
    'cover_path',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
    defaultValue: const Constant(''),
  );
  static const VerificationMeta _coverPlaceholderMeta = const VerificationMeta(
    'coverPlaceholder',
  );
  @override
  late final GeneratedColumn<String> coverPlaceholder = GeneratedColumn<String>(
    'cover_placeholder',
    aliasedName,
    true,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
  );
  static const VerificationMeta _fileSizeMeta = const VerificationMeta(
    'fileSize',
  );
  @override
  late final GeneratedColumn<int> fileSize = GeneratedColumn<int>(
    'file_size',
    aliasedName,
    false,
    type: DriftSqlType.int,
    requiredDuringInsert: false,
    defaultValue: const Constant(0),
  );
  static const VerificationMeta _createdAtMeta = const VerificationMeta(
    'createdAt',
  );
  @override
  late final GeneratedColumn<DateTime> createdAt = GeneratedColumn<DateTime>(
    'created_at',
    aliasedName,
    true,
    type: DriftSqlType.dateTime,
    requiredDuringInsert: false,
  );
  static const VerificationMeta _cachedAtMeta = const VerificationMeta(
    'cachedAt',
  );
  @override
  late final GeneratedColumn<DateTime> cachedAt = GeneratedColumn<DateTime>(
    'cached_at',
    aliasedName,
    false,
    type: DriftSqlType.dateTime,
    requiredDuringInsert: true,
  );
  @override
  List<GeneratedColumn> get $columns => [
    id,
    serverId,
    libraryId,
    title,
    seriesId,
    seriesName,
    number,
    folderPath,
    pageCount,
    coverPath,
    coverPlaceholder,
    fileSize,
    createdAt,
    cachedAt,
  ];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'cached_comics';
  @override
  VerificationContext validateIntegrity(
    Insertable<CachedComic> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('id')) {
      context.handle(_idMeta, id.isAcceptableOrUnknown(data['id']!, _idMeta));
    } else if (isInserting) {
      context.missing(_idMeta);
    }
    if (data.containsKey('server_id')) {
      context.handle(
        _serverIdMeta,
        serverId.isAcceptableOrUnknown(data['server_id']!, _serverIdMeta),
      );
    } else if (isInserting) {
      context.missing(_serverIdMeta);
    }
    if (data.containsKey('library_id')) {
      context.handle(
        _libraryIdMeta,
        libraryId.isAcceptableOrUnknown(data['library_id']!, _libraryIdMeta),
      );
    } else if (isInserting) {
      context.missing(_libraryIdMeta);
    }
    if (data.containsKey('title')) {
      context.handle(
        _titleMeta,
        title.isAcceptableOrUnknown(data['title']!, _titleMeta),
      );
    } else if (isInserting) {
      context.missing(_titleMeta);
    }
    if (data.containsKey('series_id')) {
      context.handle(
        _seriesIdMeta,
        seriesId.isAcceptableOrUnknown(data['series_id']!, _seriesIdMeta),
      );
    }
    if (data.containsKey('series_name')) {
      context.handle(
        _seriesNameMeta,
        seriesName.isAcceptableOrUnknown(data['series_name']!, _seriesNameMeta),
      );
    }
    if (data.containsKey('number')) {
      context.handle(
        _numberMeta,
        number.isAcceptableOrUnknown(data['number']!, _numberMeta),
      );
    }
    if (data.containsKey('folder_path')) {
      context.handle(
        _folderPathMeta,
        folderPath.isAcceptableOrUnknown(data['folder_path']!, _folderPathMeta),
      );
    }
    if (data.containsKey('page_count')) {
      context.handle(
        _pageCountMeta,
        pageCount.isAcceptableOrUnknown(data['page_count']!, _pageCountMeta),
      );
    }
    if (data.containsKey('cover_path')) {
      context.handle(
        _coverPathMeta,
        coverPath.isAcceptableOrUnknown(data['cover_path']!, _coverPathMeta),
      );
    }
    if (data.containsKey('cover_placeholder')) {
      context.handle(
        _coverPlaceholderMeta,
        coverPlaceholder.isAcceptableOrUnknown(
          data['cover_placeholder']!,
          _coverPlaceholderMeta,
        ),
      );
    }
    if (data.containsKey('file_size')) {
      context.handle(
        _fileSizeMeta,
        fileSize.isAcceptableOrUnknown(data['file_size']!, _fileSizeMeta),
      );
    }
    if (data.containsKey('created_at')) {
      context.handle(
        _createdAtMeta,
        createdAt.isAcceptableOrUnknown(data['created_at']!, _createdAtMeta),
      );
    }
    if (data.containsKey('cached_at')) {
      context.handle(
        _cachedAtMeta,
        cachedAt.isAcceptableOrUnknown(data['cached_at']!, _cachedAtMeta),
      );
    } else if (isInserting) {
      context.missing(_cachedAtMeta);
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {id, serverId};
  @override
  CachedComic map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return CachedComic(
      id: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}id'],
      )!,
      serverId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}server_id'],
      )!,
      libraryId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}library_id'],
      )!,
      title: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}title'],
      )!,
      seriesId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}series_id'],
      ),
      seriesName: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}series_name'],
      )!,
      number: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}number'],
      )!,
      folderPath: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}folder_path'],
      )!,
      pageCount: attachedDatabase.typeMapping.read(
        DriftSqlType.int,
        data['${effectivePrefix}page_count'],
      )!,
      coverPath: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}cover_path'],
      )!,
      coverPlaceholder: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}cover_placeholder'],
      ),
      fileSize: attachedDatabase.typeMapping.read(
        DriftSqlType.int,
        data['${effectivePrefix}file_size'],
      )!,
      createdAt: attachedDatabase.typeMapping.read(
        DriftSqlType.dateTime,
        data['${effectivePrefix}created_at'],
      ),
      cachedAt: attachedDatabase.typeMapping.read(
        DriftSqlType.dateTime,
        data['${effectivePrefix}cached_at'],
      )!,
    );
  }

  @override
  $CachedComicsTable createAlias(String alias) {
    return $CachedComicsTable(attachedDatabase, alias);
  }
}

class CachedComic extends DataClass implements Insertable<CachedComic> {
  final String id;
  final String serverId;
  final String libraryId;
  final String title;
  final String? seriesId;
  final String seriesName;
  final String number;
  final String folderPath;
  final int pageCount;
  final String coverPath;

  /// Aperçu flouté encodé en data-URI, affiché pendant le chargement.
  final String? coverPlaceholder;
  final int fileSize;

  /// Date d'ajout au catalogue, telle que le serveur la donne.
  ///
  /// Distincte de `cachedAt`, qui dit quand *cet appareil* a vu l'album. Les
  /// deux divergent dès le premier téléphone qui se connecte à une
  /// bibliothèque existante : tout y aurait été « ajouté » le même jour.
  /// Nullable pour les lignes écrites avant que la colonne n'existe ; le
  /// premier rafraîchissement les remplit.
  final DateTime? createdAt;
  final DateTime cachedAt;
  const CachedComic({
    required this.id,
    required this.serverId,
    required this.libraryId,
    required this.title,
    this.seriesId,
    required this.seriesName,
    required this.number,
    required this.folderPath,
    required this.pageCount,
    required this.coverPath,
    this.coverPlaceholder,
    required this.fileSize,
    this.createdAt,
    required this.cachedAt,
  });
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['id'] = Variable<String>(id);
    map['server_id'] = Variable<String>(serverId);
    map['library_id'] = Variable<String>(libraryId);
    map['title'] = Variable<String>(title);
    if (!nullToAbsent || seriesId != null) {
      map['series_id'] = Variable<String>(seriesId);
    }
    map['series_name'] = Variable<String>(seriesName);
    map['number'] = Variable<String>(number);
    map['folder_path'] = Variable<String>(folderPath);
    map['page_count'] = Variable<int>(pageCount);
    map['cover_path'] = Variable<String>(coverPath);
    if (!nullToAbsent || coverPlaceholder != null) {
      map['cover_placeholder'] = Variable<String>(coverPlaceholder);
    }
    map['file_size'] = Variable<int>(fileSize);
    if (!nullToAbsent || createdAt != null) {
      map['created_at'] = Variable<DateTime>(createdAt);
    }
    map['cached_at'] = Variable<DateTime>(cachedAt);
    return map;
  }

  CachedComicsCompanion toCompanion(bool nullToAbsent) {
    return CachedComicsCompanion(
      id: Value(id),
      serverId: Value(serverId),
      libraryId: Value(libraryId),
      title: Value(title),
      seriesId: seriesId == null && nullToAbsent
          ? const Value.absent()
          : Value(seriesId),
      seriesName: Value(seriesName),
      number: Value(number),
      folderPath: Value(folderPath),
      pageCount: Value(pageCount),
      coverPath: Value(coverPath),
      coverPlaceholder: coverPlaceholder == null && nullToAbsent
          ? const Value.absent()
          : Value(coverPlaceholder),
      fileSize: Value(fileSize),
      createdAt: createdAt == null && nullToAbsent
          ? const Value.absent()
          : Value(createdAt),
      cachedAt: Value(cachedAt),
    );
  }

  factory CachedComic.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return CachedComic(
      id: serializer.fromJson<String>(json['id']),
      serverId: serializer.fromJson<String>(json['serverId']),
      libraryId: serializer.fromJson<String>(json['libraryId']),
      title: serializer.fromJson<String>(json['title']),
      seriesId: serializer.fromJson<String?>(json['seriesId']),
      seriesName: serializer.fromJson<String>(json['seriesName']),
      number: serializer.fromJson<String>(json['number']),
      folderPath: serializer.fromJson<String>(json['folderPath']),
      pageCount: serializer.fromJson<int>(json['pageCount']),
      coverPath: serializer.fromJson<String>(json['coverPath']),
      coverPlaceholder: serializer.fromJson<String?>(json['coverPlaceholder']),
      fileSize: serializer.fromJson<int>(json['fileSize']),
      createdAt: serializer.fromJson<DateTime?>(json['createdAt']),
      cachedAt: serializer.fromJson<DateTime>(json['cachedAt']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'id': serializer.toJson<String>(id),
      'serverId': serializer.toJson<String>(serverId),
      'libraryId': serializer.toJson<String>(libraryId),
      'title': serializer.toJson<String>(title),
      'seriesId': serializer.toJson<String?>(seriesId),
      'seriesName': serializer.toJson<String>(seriesName),
      'number': serializer.toJson<String>(number),
      'folderPath': serializer.toJson<String>(folderPath),
      'pageCount': serializer.toJson<int>(pageCount),
      'coverPath': serializer.toJson<String>(coverPath),
      'coverPlaceholder': serializer.toJson<String?>(coverPlaceholder),
      'fileSize': serializer.toJson<int>(fileSize),
      'createdAt': serializer.toJson<DateTime?>(createdAt),
      'cachedAt': serializer.toJson<DateTime>(cachedAt),
    };
  }

  CachedComic copyWith({
    String? id,
    String? serverId,
    String? libraryId,
    String? title,
    Value<String?> seriesId = const Value.absent(),
    String? seriesName,
    String? number,
    String? folderPath,
    int? pageCount,
    String? coverPath,
    Value<String?> coverPlaceholder = const Value.absent(),
    int? fileSize,
    Value<DateTime?> createdAt = const Value.absent(),
    DateTime? cachedAt,
  }) => CachedComic(
    id: id ?? this.id,
    serverId: serverId ?? this.serverId,
    libraryId: libraryId ?? this.libraryId,
    title: title ?? this.title,
    seriesId: seriesId.present ? seriesId.value : this.seriesId,
    seriesName: seriesName ?? this.seriesName,
    number: number ?? this.number,
    folderPath: folderPath ?? this.folderPath,
    pageCount: pageCount ?? this.pageCount,
    coverPath: coverPath ?? this.coverPath,
    coverPlaceholder: coverPlaceholder.present
        ? coverPlaceholder.value
        : this.coverPlaceholder,
    fileSize: fileSize ?? this.fileSize,
    createdAt: createdAt.present ? createdAt.value : this.createdAt,
    cachedAt: cachedAt ?? this.cachedAt,
  );
  CachedComic copyWithCompanion(CachedComicsCompanion data) {
    return CachedComic(
      id: data.id.present ? data.id.value : this.id,
      serverId: data.serverId.present ? data.serverId.value : this.serverId,
      libraryId: data.libraryId.present ? data.libraryId.value : this.libraryId,
      title: data.title.present ? data.title.value : this.title,
      seriesId: data.seriesId.present ? data.seriesId.value : this.seriesId,
      seriesName: data.seriesName.present
          ? data.seriesName.value
          : this.seriesName,
      number: data.number.present ? data.number.value : this.number,
      folderPath: data.folderPath.present
          ? data.folderPath.value
          : this.folderPath,
      pageCount: data.pageCount.present ? data.pageCount.value : this.pageCount,
      coverPath: data.coverPath.present ? data.coverPath.value : this.coverPath,
      coverPlaceholder: data.coverPlaceholder.present
          ? data.coverPlaceholder.value
          : this.coverPlaceholder,
      fileSize: data.fileSize.present ? data.fileSize.value : this.fileSize,
      createdAt: data.createdAt.present ? data.createdAt.value : this.createdAt,
      cachedAt: data.cachedAt.present ? data.cachedAt.value : this.cachedAt,
    );
  }

  @override
  String toString() {
    return (StringBuffer('CachedComic(')
          ..write('id: $id, ')
          ..write('serverId: $serverId, ')
          ..write('libraryId: $libraryId, ')
          ..write('title: $title, ')
          ..write('seriesId: $seriesId, ')
          ..write('seriesName: $seriesName, ')
          ..write('number: $number, ')
          ..write('folderPath: $folderPath, ')
          ..write('pageCount: $pageCount, ')
          ..write('coverPath: $coverPath, ')
          ..write('coverPlaceholder: $coverPlaceholder, ')
          ..write('fileSize: $fileSize, ')
          ..write('createdAt: $createdAt, ')
          ..write('cachedAt: $cachedAt')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(
    id,
    serverId,
    libraryId,
    title,
    seriesId,
    seriesName,
    number,
    folderPath,
    pageCount,
    coverPath,
    coverPlaceholder,
    fileSize,
    createdAt,
    cachedAt,
  );
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is CachedComic &&
          other.id == this.id &&
          other.serverId == this.serverId &&
          other.libraryId == this.libraryId &&
          other.title == this.title &&
          other.seriesId == this.seriesId &&
          other.seriesName == this.seriesName &&
          other.number == this.number &&
          other.folderPath == this.folderPath &&
          other.pageCount == this.pageCount &&
          other.coverPath == this.coverPath &&
          other.coverPlaceholder == this.coverPlaceholder &&
          other.fileSize == this.fileSize &&
          other.createdAt == this.createdAt &&
          other.cachedAt == this.cachedAt);
}

class CachedComicsCompanion extends UpdateCompanion<CachedComic> {
  final Value<String> id;
  final Value<String> serverId;
  final Value<String> libraryId;
  final Value<String> title;
  final Value<String?> seriesId;
  final Value<String> seriesName;
  final Value<String> number;
  final Value<String> folderPath;
  final Value<int> pageCount;
  final Value<String> coverPath;
  final Value<String?> coverPlaceholder;
  final Value<int> fileSize;
  final Value<DateTime?> createdAt;
  final Value<DateTime> cachedAt;
  final Value<int> rowid;
  const CachedComicsCompanion({
    this.id = const Value.absent(),
    this.serverId = const Value.absent(),
    this.libraryId = const Value.absent(),
    this.title = const Value.absent(),
    this.seriesId = const Value.absent(),
    this.seriesName = const Value.absent(),
    this.number = const Value.absent(),
    this.folderPath = const Value.absent(),
    this.pageCount = const Value.absent(),
    this.coverPath = const Value.absent(),
    this.coverPlaceholder = const Value.absent(),
    this.fileSize = const Value.absent(),
    this.createdAt = const Value.absent(),
    this.cachedAt = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  CachedComicsCompanion.insert({
    required String id,
    required String serverId,
    required String libraryId,
    required String title,
    this.seriesId = const Value.absent(),
    this.seriesName = const Value.absent(),
    this.number = const Value.absent(),
    this.folderPath = const Value.absent(),
    this.pageCount = const Value.absent(),
    this.coverPath = const Value.absent(),
    this.coverPlaceholder = const Value.absent(),
    this.fileSize = const Value.absent(),
    this.createdAt = const Value.absent(),
    required DateTime cachedAt,
    this.rowid = const Value.absent(),
  }) : id = Value(id),
       serverId = Value(serverId),
       libraryId = Value(libraryId),
       title = Value(title),
       cachedAt = Value(cachedAt);
  static Insertable<CachedComic> custom({
    Expression<String>? id,
    Expression<String>? serverId,
    Expression<String>? libraryId,
    Expression<String>? title,
    Expression<String>? seriesId,
    Expression<String>? seriesName,
    Expression<String>? number,
    Expression<String>? folderPath,
    Expression<int>? pageCount,
    Expression<String>? coverPath,
    Expression<String>? coverPlaceholder,
    Expression<int>? fileSize,
    Expression<DateTime>? createdAt,
    Expression<DateTime>? cachedAt,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (id != null) 'id': id,
      if (serverId != null) 'server_id': serverId,
      if (libraryId != null) 'library_id': libraryId,
      if (title != null) 'title': title,
      if (seriesId != null) 'series_id': seriesId,
      if (seriesName != null) 'series_name': seriesName,
      if (number != null) 'number': number,
      if (folderPath != null) 'folder_path': folderPath,
      if (pageCount != null) 'page_count': pageCount,
      if (coverPath != null) 'cover_path': coverPath,
      if (coverPlaceholder != null) 'cover_placeholder': coverPlaceholder,
      if (fileSize != null) 'file_size': fileSize,
      if (createdAt != null) 'created_at': createdAt,
      if (cachedAt != null) 'cached_at': cachedAt,
      if (rowid != null) 'rowid': rowid,
    });
  }

  CachedComicsCompanion copyWith({
    Value<String>? id,
    Value<String>? serverId,
    Value<String>? libraryId,
    Value<String>? title,
    Value<String?>? seriesId,
    Value<String>? seriesName,
    Value<String>? number,
    Value<String>? folderPath,
    Value<int>? pageCount,
    Value<String>? coverPath,
    Value<String?>? coverPlaceholder,
    Value<int>? fileSize,
    Value<DateTime?>? createdAt,
    Value<DateTime>? cachedAt,
    Value<int>? rowid,
  }) {
    return CachedComicsCompanion(
      id: id ?? this.id,
      serverId: serverId ?? this.serverId,
      libraryId: libraryId ?? this.libraryId,
      title: title ?? this.title,
      seriesId: seriesId ?? this.seriesId,
      seriesName: seriesName ?? this.seriesName,
      number: number ?? this.number,
      folderPath: folderPath ?? this.folderPath,
      pageCount: pageCount ?? this.pageCount,
      coverPath: coverPath ?? this.coverPath,
      coverPlaceholder: coverPlaceholder ?? this.coverPlaceholder,
      fileSize: fileSize ?? this.fileSize,
      createdAt: createdAt ?? this.createdAt,
      cachedAt: cachedAt ?? this.cachedAt,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (id.present) {
      map['id'] = Variable<String>(id.value);
    }
    if (serverId.present) {
      map['server_id'] = Variable<String>(serverId.value);
    }
    if (libraryId.present) {
      map['library_id'] = Variable<String>(libraryId.value);
    }
    if (title.present) {
      map['title'] = Variable<String>(title.value);
    }
    if (seriesId.present) {
      map['series_id'] = Variable<String>(seriesId.value);
    }
    if (seriesName.present) {
      map['series_name'] = Variable<String>(seriesName.value);
    }
    if (number.present) {
      map['number'] = Variable<String>(number.value);
    }
    if (folderPath.present) {
      map['folder_path'] = Variable<String>(folderPath.value);
    }
    if (pageCount.present) {
      map['page_count'] = Variable<int>(pageCount.value);
    }
    if (coverPath.present) {
      map['cover_path'] = Variable<String>(coverPath.value);
    }
    if (coverPlaceholder.present) {
      map['cover_placeholder'] = Variable<String>(coverPlaceholder.value);
    }
    if (fileSize.present) {
      map['file_size'] = Variable<int>(fileSize.value);
    }
    if (createdAt.present) {
      map['created_at'] = Variable<DateTime>(createdAt.value);
    }
    if (cachedAt.present) {
      map['cached_at'] = Variable<DateTime>(cachedAt.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('CachedComicsCompanion(')
          ..write('id: $id, ')
          ..write('serverId: $serverId, ')
          ..write('libraryId: $libraryId, ')
          ..write('title: $title, ')
          ..write('seriesId: $seriesId, ')
          ..write('seriesName: $seriesName, ')
          ..write('number: $number, ')
          ..write('folderPath: $folderPath, ')
          ..write('pageCount: $pageCount, ')
          ..write('coverPath: $coverPath, ')
          ..write('coverPlaceholder: $coverPlaceholder, ')
          ..write('fileSize: $fileSize, ')
          ..write('createdAt: $createdAt, ')
          ..write('cachedAt: $cachedAt, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

class $CachedSeriesTable extends CachedSeries
    with TableInfo<$CachedSeriesTable, CachedSeriesRow> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $CachedSeriesTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _idMeta = const VerificationMeta('id');
  @override
  late final GeneratedColumn<String> id = GeneratedColumn<String>(
    'id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _serverIdMeta = const VerificationMeta(
    'serverId',
  );
  @override
  late final GeneratedColumn<String> serverId = GeneratedColumn<String>(
    'server_id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _libraryIdMeta = const VerificationMeta(
    'libraryId',
  );
  @override
  late final GeneratedColumn<String> libraryId = GeneratedColumn<String>(
    'library_id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _nameMeta = const VerificationMeta('name');
  @override
  late final GeneratedColumn<String> name = GeneratedColumn<String>(
    'name',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _comicCountMeta = const VerificationMeta(
    'comicCount',
  );
  @override
  late final GeneratedColumn<int> comicCount = GeneratedColumn<int>(
    'comic_count',
    aliasedName,
    false,
    type: DriftSqlType.int,
    requiredDuringInsert: false,
    defaultValue: const Constant(0),
  );
  static const VerificationMeta _coverPathMeta = const VerificationMeta(
    'coverPath',
  );
  @override
  late final GeneratedColumn<String> coverPath = GeneratedColumn<String>(
    'cover_path',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
    defaultValue: const Constant(''),
  );
  @override
  List<GeneratedColumn> get $columns => [
    id,
    serverId,
    libraryId,
    name,
    comicCount,
    coverPath,
  ];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'cached_series';
  @override
  VerificationContext validateIntegrity(
    Insertable<CachedSeriesRow> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('id')) {
      context.handle(_idMeta, id.isAcceptableOrUnknown(data['id']!, _idMeta));
    } else if (isInserting) {
      context.missing(_idMeta);
    }
    if (data.containsKey('server_id')) {
      context.handle(
        _serverIdMeta,
        serverId.isAcceptableOrUnknown(data['server_id']!, _serverIdMeta),
      );
    } else if (isInserting) {
      context.missing(_serverIdMeta);
    }
    if (data.containsKey('library_id')) {
      context.handle(
        _libraryIdMeta,
        libraryId.isAcceptableOrUnknown(data['library_id']!, _libraryIdMeta),
      );
    } else if (isInserting) {
      context.missing(_libraryIdMeta);
    }
    if (data.containsKey('name')) {
      context.handle(
        _nameMeta,
        name.isAcceptableOrUnknown(data['name']!, _nameMeta),
      );
    } else if (isInserting) {
      context.missing(_nameMeta);
    }
    if (data.containsKey('comic_count')) {
      context.handle(
        _comicCountMeta,
        comicCount.isAcceptableOrUnknown(data['comic_count']!, _comicCountMeta),
      );
    }
    if (data.containsKey('cover_path')) {
      context.handle(
        _coverPathMeta,
        coverPath.isAcceptableOrUnknown(data['cover_path']!, _coverPathMeta),
      );
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {id, serverId};
  @override
  CachedSeriesRow map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return CachedSeriesRow(
      id: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}id'],
      )!,
      serverId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}server_id'],
      )!,
      libraryId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}library_id'],
      )!,
      name: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}name'],
      )!,
      comicCount: attachedDatabase.typeMapping.read(
        DriftSqlType.int,
        data['${effectivePrefix}comic_count'],
      )!,
      coverPath: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}cover_path'],
      )!,
    );
  }

  @override
  $CachedSeriesTable createAlias(String alias) {
    return $CachedSeriesTable(attachedDatabase, alias);
  }
}

class CachedSeriesRow extends DataClass implements Insertable<CachedSeriesRow> {
  final String id;
  final String serverId;
  final String libraryId;
  final String name;
  final int comicCount;
  final String coverPath;
  const CachedSeriesRow({
    required this.id,
    required this.serverId,
    required this.libraryId,
    required this.name,
    required this.comicCount,
    required this.coverPath,
  });
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['id'] = Variable<String>(id);
    map['server_id'] = Variable<String>(serverId);
    map['library_id'] = Variable<String>(libraryId);
    map['name'] = Variable<String>(name);
    map['comic_count'] = Variable<int>(comicCount);
    map['cover_path'] = Variable<String>(coverPath);
    return map;
  }

  CachedSeriesCompanion toCompanion(bool nullToAbsent) {
    return CachedSeriesCompanion(
      id: Value(id),
      serverId: Value(serverId),
      libraryId: Value(libraryId),
      name: Value(name),
      comicCount: Value(comicCount),
      coverPath: Value(coverPath),
    );
  }

  factory CachedSeriesRow.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return CachedSeriesRow(
      id: serializer.fromJson<String>(json['id']),
      serverId: serializer.fromJson<String>(json['serverId']),
      libraryId: serializer.fromJson<String>(json['libraryId']),
      name: serializer.fromJson<String>(json['name']),
      comicCount: serializer.fromJson<int>(json['comicCount']),
      coverPath: serializer.fromJson<String>(json['coverPath']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'id': serializer.toJson<String>(id),
      'serverId': serializer.toJson<String>(serverId),
      'libraryId': serializer.toJson<String>(libraryId),
      'name': serializer.toJson<String>(name),
      'comicCount': serializer.toJson<int>(comicCount),
      'coverPath': serializer.toJson<String>(coverPath),
    };
  }

  CachedSeriesRow copyWith({
    String? id,
    String? serverId,
    String? libraryId,
    String? name,
    int? comicCount,
    String? coverPath,
  }) => CachedSeriesRow(
    id: id ?? this.id,
    serverId: serverId ?? this.serverId,
    libraryId: libraryId ?? this.libraryId,
    name: name ?? this.name,
    comicCount: comicCount ?? this.comicCount,
    coverPath: coverPath ?? this.coverPath,
  );
  CachedSeriesRow copyWithCompanion(CachedSeriesCompanion data) {
    return CachedSeriesRow(
      id: data.id.present ? data.id.value : this.id,
      serverId: data.serverId.present ? data.serverId.value : this.serverId,
      libraryId: data.libraryId.present ? data.libraryId.value : this.libraryId,
      name: data.name.present ? data.name.value : this.name,
      comicCount: data.comicCount.present
          ? data.comicCount.value
          : this.comicCount,
      coverPath: data.coverPath.present ? data.coverPath.value : this.coverPath,
    );
  }

  @override
  String toString() {
    return (StringBuffer('CachedSeriesRow(')
          ..write('id: $id, ')
          ..write('serverId: $serverId, ')
          ..write('libraryId: $libraryId, ')
          ..write('name: $name, ')
          ..write('comicCount: $comicCount, ')
          ..write('coverPath: $coverPath')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode =>
      Object.hash(id, serverId, libraryId, name, comicCount, coverPath);
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is CachedSeriesRow &&
          other.id == this.id &&
          other.serverId == this.serverId &&
          other.libraryId == this.libraryId &&
          other.name == this.name &&
          other.comicCount == this.comicCount &&
          other.coverPath == this.coverPath);
}

class CachedSeriesCompanion extends UpdateCompanion<CachedSeriesRow> {
  final Value<String> id;
  final Value<String> serverId;
  final Value<String> libraryId;
  final Value<String> name;
  final Value<int> comicCount;
  final Value<String> coverPath;
  final Value<int> rowid;
  const CachedSeriesCompanion({
    this.id = const Value.absent(),
    this.serverId = const Value.absent(),
    this.libraryId = const Value.absent(),
    this.name = const Value.absent(),
    this.comicCount = const Value.absent(),
    this.coverPath = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  CachedSeriesCompanion.insert({
    required String id,
    required String serverId,
    required String libraryId,
    required String name,
    this.comicCount = const Value.absent(),
    this.coverPath = const Value.absent(),
    this.rowid = const Value.absent(),
  }) : id = Value(id),
       serverId = Value(serverId),
       libraryId = Value(libraryId),
       name = Value(name);
  static Insertable<CachedSeriesRow> custom({
    Expression<String>? id,
    Expression<String>? serverId,
    Expression<String>? libraryId,
    Expression<String>? name,
    Expression<int>? comicCount,
    Expression<String>? coverPath,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (id != null) 'id': id,
      if (serverId != null) 'server_id': serverId,
      if (libraryId != null) 'library_id': libraryId,
      if (name != null) 'name': name,
      if (comicCount != null) 'comic_count': comicCount,
      if (coverPath != null) 'cover_path': coverPath,
      if (rowid != null) 'rowid': rowid,
    });
  }

  CachedSeriesCompanion copyWith({
    Value<String>? id,
    Value<String>? serverId,
    Value<String>? libraryId,
    Value<String>? name,
    Value<int>? comicCount,
    Value<String>? coverPath,
    Value<int>? rowid,
  }) {
    return CachedSeriesCompanion(
      id: id ?? this.id,
      serverId: serverId ?? this.serverId,
      libraryId: libraryId ?? this.libraryId,
      name: name ?? this.name,
      comicCount: comicCount ?? this.comicCount,
      coverPath: coverPath ?? this.coverPath,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (id.present) {
      map['id'] = Variable<String>(id.value);
    }
    if (serverId.present) {
      map['server_id'] = Variable<String>(serverId.value);
    }
    if (libraryId.present) {
      map['library_id'] = Variable<String>(libraryId.value);
    }
    if (name.present) {
      map['name'] = Variable<String>(name.value);
    }
    if (comicCount.present) {
      map['comic_count'] = Variable<int>(comicCount.value);
    }
    if (coverPath.present) {
      map['cover_path'] = Variable<String>(coverPath.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('CachedSeriesCompanion(')
          ..write('id: $id, ')
          ..write('serverId: $serverId, ')
          ..write('libraryId: $libraryId, ')
          ..write('name: $name, ')
          ..write('comicCount: $comicCount, ')
          ..write('coverPath: $coverPath, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

class $CachedFoldersTable extends CachedFolders
    with TableInfo<$CachedFoldersTable, CachedFolder> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $CachedFoldersTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _serverIdMeta = const VerificationMeta(
    'serverId',
  );
  @override
  late final GeneratedColumn<String> serverId = GeneratedColumn<String>(
    'server_id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _libraryIdMeta = const VerificationMeta(
    'libraryId',
  );
  @override
  late final GeneratedColumn<String> libraryId = GeneratedColumn<String>(
    'library_id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _pathMeta = const VerificationMeta('path');
  @override
  late final GeneratedColumn<String> path = GeneratedColumn<String>(
    'path',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _nameMeta = const VerificationMeta('name');
  @override
  late final GeneratedColumn<String> name = GeneratedColumn<String>(
    'name',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _depthMeta = const VerificationMeta('depth');
  @override
  late final GeneratedColumn<int> depth = GeneratedColumn<int>(
    'depth',
    aliasedName,
    false,
    type: DriftSqlType.int,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _comicCountMeta = const VerificationMeta(
    'comicCount',
  );
  @override
  late final GeneratedColumn<int> comicCount = GeneratedColumn<int>(
    'comic_count',
    aliasedName,
    false,
    type: DriftSqlType.int,
    requiredDuringInsert: false,
    defaultValue: const Constant(0),
  );
  @override
  List<GeneratedColumn> get $columns => [
    serverId,
    libraryId,
    path,
    name,
    depth,
    comicCount,
  ];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'cached_folders';
  @override
  VerificationContext validateIntegrity(
    Insertable<CachedFolder> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('server_id')) {
      context.handle(
        _serverIdMeta,
        serverId.isAcceptableOrUnknown(data['server_id']!, _serverIdMeta),
      );
    } else if (isInserting) {
      context.missing(_serverIdMeta);
    }
    if (data.containsKey('library_id')) {
      context.handle(
        _libraryIdMeta,
        libraryId.isAcceptableOrUnknown(data['library_id']!, _libraryIdMeta),
      );
    } else if (isInserting) {
      context.missing(_libraryIdMeta);
    }
    if (data.containsKey('path')) {
      context.handle(
        _pathMeta,
        path.isAcceptableOrUnknown(data['path']!, _pathMeta),
      );
    } else if (isInserting) {
      context.missing(_pathMeta);
    }
    if (data.containsKey('name')) {
      context.handle(
        _nameMeta,
        name.isAcceptableOrUnknown(data['name']!, _nameMeta),
      );
    } else if (isInserting) {
      context.missing(_nameMeta);
    }
    if (data.containsKey('depth')) {
      context.handle(
        _depthMeta,
        depth.isAcceptableOrUnknown(data['depth']!, _depthMeta),
      );
    } else if (isInserting) {
      context.missing(_depthMeta);
    }
    if (data.containsKey('comic_count')) {
      context.handle(
        _comicCountMeta,
        comicCount.isAcceptableOrUnknown(data['comic_count']!, _comicCountMeta),
      );
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {serverId, libraryId, path};
  @override
  CachedFolder map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return CachedFolder(
      serverId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}server_id'],
      )!,
      libraryId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}library_id'],
      )!,
      path: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}path'],
      )!,
      name: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}name'],
      )!,
      depth: attachedDatabase.typeMapping.read(
        DriftSqlType.int,
        data['${effectivePrefix}depth'],
      )!,
      comicCount: attachedDatabase.typeMapping.read(
        DriftSqlType.int,
        data['${effectivePrefix}comic_count'],
      )!,
    );
  }

  @override
  $CachedFoldersTable createAlias(String alias) {
    return $CachedFoldersTable(attachedDatabase, alias);
  }
}

class CachedFolder extends DataClass implements Insertable<CachedFolder> {
  final String serverId;
  final String libraryId;
  final String path;
  final String name;
  final int depth;
  final int comicCount;
  const CachedFolder({
    required this.serverId,
    required this.libraryId,
    required this.path,
    required this.name,
    required this.depth,
    required this.comicCount,
  });
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['server_id'] = Variable<String>(serverId);
    map['library_id'] = Variable<String>(libraryId);
    map['path'] = Variable<String>(path);
    map['name'] = Variable<String>(name);
    map['depth'] = Variable<int>(depth);
    map['comic_count'] = Variable<int>(comicCount);
    return map;
  }

  CachedFoldersCompanion toCompanion(bool nullToAbsent) {
    return CachedFoldersCompanion(
      serverId: Value(serverId),
      libraryId: Value(libraryId),
      path: Value(path),
      name: Value(name),
      depth: Value(depth),
      comicCount: Value(comicCount),
    );
  }

  factory CachedFolder.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return CachedFolder(
      serverId: serializer.fromJson<String>(json['serverId']),
      libraryId: serializer.fromJson<String>(json['libraryId']),
      path: serializer.fromJson<String>(json['path']),
      name: serializer.fromJson<String>(json['name']),
      depth: serializer.fromJson<int>(json['depth']),
      comicCount: serializer.fromJson<int>(json['comicCount']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'serverId': serializer.toJson<String>(serverId),
      'libraryId': serializer.toJson<String>(libraryId),
      'path': serializer.toJson<String>(path),
      'name': serializer.toJson<String>(name),
      'depth': serializer.toJson<int>(depth),
      'comicCount': serializer.toJson<int>(comicCount),
    };
  }

  CachedFolder copyWith({
    String? serverId,
    String? libraryId,
    String? path,
    String? name,
    int? depth,
    int? comicCount,
  }) => CachedFolder(
    serverId: serverId ?? this.serverId,
    libraryId: libraryId ?? this.libraryId,
    path: path ?? this.path,
    name: name ?? this.name,
    depth: depth ?? this.depth,
    comicCount: comicCount ?? this.comicCount,
  );
  CachedFolder copyWithCompanion(CachedFoldersCompanion data) {
    return CachedFolder(
      serverId: data.serverId.present ? data.serverId.value : this.serverId,
      libraryId: data.libraryId.present ? data.libraryId.value : this.libraryId,
      path: data.path.present ? data.path.value : this.path,
      name: data.name.present ? data.name.value : this.name,
      depth: data.depth.present ? data.depth.value : this.depth,
      comicCount: data.comicCount.present
          ? data.comicCount.value
          : this.comicCount,
    );
  }

  @override
  String toString() {
    return (StringBuffer('CachedFolder(')
          ..write('serverId: $serverId, ')
          ..write('libraryId: $libraryId, ')
          ..write('path: $path, ')
          ..write('name: $name, ')
          ..write('depth: $depth, ')
          ..write('comicCount: $comicCount')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode =>
      Object.hash(serverId, libraryId, path, name, depth, comicCount);
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is CachedFolder &&
          other.serverId == this.serverId &&
          other.libraryId == this.libraryId &&
          other.path == this.path &&
          other.name == this.name &&
          other.depth == this.depth &&
          other.comicCount == this.comicCount);
}

class CachedFoldersCompanion extends UpdateCompanion<CachedFolder> {
  final Value<String> serverId;
  final Value<String> libraryId;
  final Value<String> path;
  final Value<String> name;
  final Value<int> depth;
  final Value<int> comicCount;
  final Value<int> rowid;
  const CachedFoldersCompanion({
    this.serverId = const Value.absent(),
    this.libraryId = const Value.absent(),
    this.path = const Value.absent(),
    this.name = const Value.absent(),
    this.depth = const Value.absent(),
    this.comicCount = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  CachedFoldersCompanion.insert({
    required String serverId,
    required String libraryId,
    required String path,
    required String name,
    required int depth,
    this.comicCount = const Value.absent(),
    this.rowid = const Value.absent(),
  }) : serverId = Value(serverId),
       libraryId = Value(libraryId),
       path = Value(path),
       name = Value(name),
       depth = Value(depth);
  static Insertable<CachedFolder> custom({
    Expression<String>? serverId,
    Expression<String>? libraryId,
    Expression<String>? path,
    Expression<String>? name,
    Expression<int>? depth,
    Expression<int>? comicCount,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (serverId != null) 'server_id': serverId,
      if (libraryId != null) 'library_id': libraryId,
      if (path != null) 'path': path,
      if (name != null) 'name': name,
      if (depth != null) 'depth': depth,
      if (comicCount != null) 'comic_count': comicCount,
      if (rowid != null) 'rowid': rowid,
    });
  }

  CachedFoldersCompanion copyWith({
    Value<String>? serverId,
    Value<String>? libraryId,
    Value<String>? path,
    Value<String>? name,
    Value<int>? depth,
    Value<int>? comicCount,
    Value<int>? rowid,
  }) {
    return CachedFoldersCompanion(
      serverId: serverId ?? this.serverId,
      libraryId: libraryId ?? this.libraryId,
      path: path ?? this.path,
      name: name ?? this.name,
      depth: depth ?? this.depth,
      comicCount: comicCount ?? this.comicCount,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (serverId.present) {
      map['server_id'] = Variable<String>(serverId.value);
    }
    if (libraryId.present) {
      map['library_id'] = Variable<String>(libraryId.value);
    }
    if (path.present) {
      map['path'] = Variable<String>(path.value);
    }
    if (name.present) {
      map['name'] = Variable<String>(name.value);
    }
    if (depth.present) {
      map['depth'] = Variable<int>(depth.value);
    }
    if (comicCount.present) {
      map['comic_count'] = Variable<int>(comicCount.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('CachedFoldersCompanion(')
          ..write('serverId: $serverId, ')
          ..write('libraryId: $libraryId, ')
          ..write('path: $path, ')
          ..write('name: $name, ')
          ..write('depth: $depth, ')
          ..write('comicCount: $comicCount, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

class $CachedLibrariesTable extends CachedLibraries
    with TableInfo<$CachedLibrariesTable, CachedLibrary> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $CachedLibrariesTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _idMeta = const VerificationMeta('id');
  @override
  late final GeneratedColumn<String> id = GeneratedColumn<String>(
    'id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _serverIdMeta = const VerificationMeta(
    'serverId',
  );
  @override
  late final GeneratedColumn<String> serverId = GeneratedColumn<String>(
    'server_id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _nameMeta = const VerificationMeta('name');
  @override
  late final GeneratedColumn<String> name = GeneratedColumn<String>(
    'name',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _comicCountMeta = const VerificationMeta(
    'comicCount',
  );
  @override
  late final GeneratedColumn<int> comicCount = GeneratedColumn<int>(
    'comic_count',
    aliasedName,
    false,
    type: DriftSqlType.int,
    requiredDuringInsert: false,
    defaultValue: const Constant(0),
  );
  @override
  List<GeneratedColumn> get $columns => [id, serverId, name, comicCount];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'cached_libraries';
  @override
  VerificationContext validateIntegrity(
    Insertable<CachedLibrary> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('id')) {
      context.handle(_idMeta, id.isAcceptableOrUnknown(data['id']!, _idMeta));
    } else if (isInserting) {
      context.missing(_idMeta);
    }
    if (data.containsKey('server_id')) {
      context.handle(
        _serverIdMeta,
        serverId.isAcceptableOrUnknown(data['server_id']!, _serverIdMeta),
      );
    } else if (isInserting) {
      context.missing(_serverIdMeta);
    }
    if (data.containsKey('name')) {
      context.handle(
        _nameMeta,
        name.isAcceptableOrUnknown(data['name']!, _nameMeta),
      );
    } else if (isInserting) {
      context.missing(_nameMeta);
    }
    if (data.containsKey('comic_count')) {
      context.handle(
        _comicCountMeta,
        comicCount.isAcceptableOrUnknown(data['comic_count']!, _comicCountMeta),
      );
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {id, serverId};
  @override
  CachedLibrary map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return CachedLibrary(
      id: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}id'],
      )!,
      serverId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}server_id'],
      )!,
      name: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}name'],
      )!,
      comicCount: attachedDatabase.typeMapping.read(
        DriftSqlType.int,
        data['${effectivePrefix}comic_count'],
      )!,
    );
  }

  @override
  $CachedLibrariesTable createAlias(String alias) {
    return $CachedLibrariesTable(attachedDatabase, alias);
  }
}

class CachedLibrary extends DataClass implements Insertable<CachedLibrary> {
  final String id;
  final String serverId;
  final String name;
  final int comicCount;
  const CachedLibrary({
    required this.id,
    required this.serverId,
    required this.name,
    required this.comicCount,
  });
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['id'] = Variable<String>(id);
    map['server_id'] = Variable<String>(serverId);
    map['name'] = Variable<String>(name);
    map['comic_count'] = Variable<int>(comicCount);
    return map;
  }

  CachedLibrariesCompanion toCompanion(bool nullToAbsent) {
    return CachedLibrariesCompanion(
      id: Value(id),
      serverId: Value(serverId),
      name: Value(name),
      comicCount: Value(comicCount),
    );
  }

  factory CachedLibrary.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return CachedLibrary(
      id: serializer.fromJson<String>(json['id']),
      serverId: serializer.fromJson<String>(json['serverId']),
      name: serializer.fromJson<String>(json['name']),
      comicCount: serializer.fromJson<int>(json['comicCount']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'id': serializer.toJson<String>(id),
      'serverId': serializer.toJson<String>(serverId),
      'name': serializer.toJson<String>(name),
      'comicCount': serializer.toJson<int>(comicCount),
    };
  }

  CachedLibrary copyWith({
    String? id,
    String? serverId,
    String? name,
    int? comicCount,
  }) => CachedLibrary(
    id: id ?? this.id,
    serverId: serverId ?? this.serverId,
    name: name ?? this.name,
    comicCount: comicCount ?? this.comicCount,
  );
  CachedLibrary copyWithCompanion(CachedLibrariesCompanion data) {
    return CachedLibrary(
      id: data.id.present ? data.id.value : this.id,
      serverId: data.serverId.present ? data.serverId.value : this.serverId,
      name: data.name.present ? data.name.value : this.name,
      comicCount: data.comicCount.present
          ? data.comicCount.value
          : this.comicCount,
    );
  }

  @override
  String toString() {
    return (StringBuffer('CachedLibrary(')
          ..write('id: $id, ')
          ..write('serverId: $serverId, ')
          ..write('name: $name, ')
          ..write('comicCount: $comicCount')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(id, serverId, name, comicCount);
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is CachedLibrary &&
          other.id == this.id &&
          other.serverId == this.serverId &&
          other.name == this.name &&
          other.comicCount == this.comicCount);
}

class CachedLibrariesCompanion extends UpdateCompanion<CachedLibrary> {
  final Value<String> id;
  final Value<String> serverId;
  final Value<String> name;
  final Value<int> comicCount;
  final Value<int> rowid;
  const CachedLibrariesCompanion({
    this.id = const Value.absent(),
    this.serverId = const Value.absent(),
    this.name = const Value.absent(),
    this.comicCount = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  CachedLibrariesCompanion.insert({
    required String id,
    required String serverId,
    required String name,
    this.comicCount = const Value.absent(),
    this.rowid = const Value.absent(),
  }) : id = Value(id),
       serverId = Value(serverId),
       name = Value(name);
  static Insertable<CachedLibrary> custom({
    Expression<String>? id,
    Expression<String>? serverId,
    Expression<String>? name,
    Expression<int>? comicCount,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (id != null) 'id': id,
      if (serverId != null) 'server_id': serverId,
      if (name != null) 'name': name,
      if (comicCount != null) 'comic_count': comicCount,
      if (rowid != null) 'rowid': rowid,
    });
  }

  CachedLibrariesCompanion copyWith({
    Value<String>? id,
    Value<String>? serverId,
    Value<String>? name,
    Value<int>? comicCount,
    Value<int>? rowid,
  }) {
    return CachedLibrariesCompanion(
      id: id ?? this.id,
      serverId: serverId ?? this.serverId,
      name: name ?? this.name,
      comicCount: comicCount ?? this.comicCount,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (id.present) {
      map['id'] = Variable<String>(id.value);
    }
    if (serverId.present) {
      map['server_id'] = Variable<String>(serverId.value);
    }
    if (name.present) {
      map['name'] = Variable<String>(name.value);
    }
    if (comicCount.present) {
      map['comic_count'] = Variable<int>(comicCount.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('CachedLibrariesCompanion(')
          ..write('id: $id, ')
          ..write('serverId: $serverId, ')
          ..write('name: $name, ')
          ..write('comicCount: $comicCount, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

class $LocalProgressTable extends LocalProgress
    with TableInfo<$LocalProgressTable, LocalProgressData> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $LocalProgressTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _comicIdMeta = const VerificationMeta(
    'comicId',
  );
  @override
  late final GeneratedColumn<String> comicId = GeneratedColumn<String>(
    'comic_id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _serverIdMeta = const VerificationMeta(
    'serverId',
  );
  @override
  late final GeneratedColumn<String> serverId = GeneratedColumn<String>(
    'server_id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _pageMeta = const VerificationMeta('page');
  @override
  late final GeneratedColumn<int> page = GeneratedColumn<int>(
    'page',
    aliasedName,
    false,
    type: DriftSqlType.int,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _pageCountMeta = const VerificationMeta(
    'pageCount',
  );
  @override
  late final GeneratedColumn<int> pageCount = GeneratedColumn<int>(
    'page_count',
    aliasedName,
    false,
    type: DriftSqlType.int,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _statusMeta = const VerificationMeta('status');
  @override
  late final GeneratedColumn<String> status = GeneratedColumn<String>(
    'status',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _updatedAtMeta = const VerificationMeta(
    'updatedAt',
  );
  @override
  late final GeneratedColumn<DateTime> updatedAt = GeneratedColumn<DateTime>(
    'updated_at',
    aliasedName,
    false,
    type: DriftSqlType.dateTime,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _pendingMeta = const VerificationMeta(
    'pending',
  );
  @override
  late final GeneratedColumn<bool> pending = GeneratedColumn<bool>(
    'pending',
    aliasedName,
    false,
    type: DriftSqlType.bool,
    requiredDuringInsert: false,
    defaultConstraints: GeneratedColumn.constraintIsAlways(
      'CHECK ("pending" IN (0, 1))',
    ),
    defaultValue: const Constant(false),
  );
  @override
  List<GeneratedColumn> get $columns => [
    comicId,
    serverId,
    page,
    pageCount,
    status,
    updatedAt,
    pending,
  ];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'local_progress';
  @override
  VerificationContext validateIntegrity(
    Insertable<LocalProgressData> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('comic_id')) {
      context.handle(
        _comicIdMeta,
        comicId.isAcceptableOrUnknown(data['comic_id']!, _comicIdMeta),
      );
    } else if (isInserting) {
      context.missing(_comicIdMeta);
    }
    if (data.containsKey('server_id')) {
      context.handle(
        _serverIdMeta,
        serverId.isAcceptableOrUnknown(data['server_id']!, _serverIdMeta),
      );
    } else if (isInserting) {
      context.missing(_serverIdMeta);
    }
    if (data.containsKey('page')) {
      context.handle(
        _pageMeta,
        page.isAcceptableOrUnknown(data['page']!, _pageMeta),
      );
    } else if (isInserting) {
      context.missing(_pageMeta);
    }
    if (data.containsKey('page_count')) {
      context.handle(
        _pageCountMeta,
        pageCount.isAcceptableOrUnknown(data['page_count']!, _pageCountMeta),
      );
    } else if (isInserting) {
      context.missing(_pageCountMeta);
    }
    if (data.containsKey('status')) {
      context.handle(
        _statusMeta,
        status.isAcceptableOrUnknown(data['status']!, _statusMeta),
      );
    } else if (isInserting) {
      context.missing(_statusMeta);
    }
    if (data.containsKey('updated_at')) {
      context.handle(
        _updatedAtMeta,
        updatedAt.isAcceptableOrUnknown(data['updated_at']!, _updatedAtMeta),
      );
    } else if (isInserting) {
      context.missing(_updatedAtMeta);
    }
    if (data.containsKey('pending')) {
      context.handle(
        _pendingMeta,
        pending.isAcceptableOrUnknown(data['pending']!, _pendingMeta),
      );
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {comicId, serverId};
  @override
  LocalProgressData map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return LocalProgressData(
      comicId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}comic_id'],
      )!,
      serverId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}server_id'],
      )!,
      page: attachedDatabase.typeMapping.read(
        DriftSqlType.int,
        data['${effectivePrefix}page'],
      )!,
      pageCount: attachedDatabase.typeMapping.read(
        DriftSqlType.int,
        data['${effectivePrefix}page_count'],
      )!,
      status: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}status'],
      )!,
      updatedAt: attachedDatabase.typeMapping.read(
        DriftSqlType.dateTime,
        data['${effectivePrefix}updated_at'],
      )!,
      pending: attachedDatabase.typeMapping.read(
        DriftSqlType.bool,
        data['${effectivePrefix}pending'],
      )!,
    );
  }

  @override
  $LocalProgressTable createAlias(String alias) {
    return $LocalProgressTable(attachedDatabase, alias);
  }
}

class LocalProgressData extends DataClass
    implements Insertable<LocalProgressData> {
  final String comicId;
  final String serverId;
  final int page;
  final int pageCount;
  final String status;
  final DateTime updatedAt;

  /// Vrai tant que le serveur n'a pas confirmé cette progression.
  final bool pending;
  const LocalProgressData({
    required this.comicId,
    required this.serverId,
    required this.page,
    required this.pageCount,
    required this.status,
    required this.updatedAt,
    required this.pending,
  });
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['comic_id'] = Variable<String>(comicId);
    map['server_id'] = Variable<String>(serverId);
    map['page'] = Variable<int>(page);
    map['page_count'] = Variable<int>(pageCount);
    map['status'] = Variable<String>(status);
    map['updated_at'] = Variable<DateTime>(updatedAt);
    map['pending'] = Variable<bool>(pending);
    return map;
  }

  LocalProgressCompanion toCompanion(bool nullToAbsent) {
    return LocalProgressCompanion(
      comicId: Value(comicId),
      serverId: Value(serverId),
      page: Value(page),
      pageCount: Value(pageCount),
      status: Value(status),
      updatedAt: Value(updatedAt),
      pending: Value(pending),
    );
  }

  factory LocalProgressData.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return LocalProgressData(
      comicId: serializer.fromJson<String>(json['comicId']),
      serverId: serializer.fromJson<String>(json['serverId']),
      page: serializer.fromJson<int>(json['page']),
      pageCount: serializer.fromJson<int>(json['pageCount']),
      status: serializer.fromJson<String>(json['status']),
      updatedAt: serializer.fromJson<DateTime>(json['updatedAt']),
      pending: serializer.fromJson<bool>(json['pending']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'comicId': serializer.toJson<String>(comicId),
      'serverId': serializer.toJson<String>(serverId),
      'page': serializer.toJson<int>(page),
      'pageCount': serializer.toJson<int>(pageCount),
      'status': serializer.toJson<String>(status),
      'updatedAt': serializer.toJson<DateTime>(updatedAt),
      'pending': serializer.toJson<bool>(pending),
    };
  }

  LocalProgressData copyWith({
    String? comicId,
    String? serverId,
    int? page,
    int? pageCount,
    String? status,
    DateTime? updatedAt,
    bool? pending,
  }) => LocalProgressData(
    comicId: comicId ?? this.comicId,
    serverId: serverId ?? this.serverId,
    page: page ?? this.page,
    pageCount: pageCount ?? this.pageCount,
    status: status ?? this.status,
    updatedAt: updatedAt ?? this.updatedAt,
    pending: pending ?? this.pending,
  );
  LocalProgressData copyWithCompanion(LocalProgressCompanion data) {
    return LocalProgressData(
      comicId: data.comicId.present ? data.comicId.value : this.comicId,
      serverId: data.serverId.present ? data.serverId.value : this.serverId,
      page: data.page.present ? data.page.value : this.page,
      pageCount: data.pageCount.present ? data.pageCount.value : this.pageCount,
      status: data.status.present ? data.status.value : this.status,
      updatedAt: data.updatedAt.present ? data.updatedAt.value : this.updatedAt,
      pending: data.pending.present ? data.pending.value : this.pending,
    );
  }

  @override
  String toString() {
    return (StringBuffer('LocalProgressData(')
          ..write('comicId: $comicId, ')
          ..write('serverId: $serverId, ')
          ..write('page: $page, ')
          ..write('pageCount: $pageCount, ')
          ..write('status: $status, ')
          ..write('updatedAt: $updatedAt, ')
          ..write('pending: $pending')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(
    comicId,
    serverId,
    page,
    pageCount,
    status,
    updatedAt,
    pending,
  );
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is LocalProgressData &&
          other.comicId == this.comicId &&
          other.serverId == this.serverId &&
          other.page == this.page &&
          other.pageCount == this.pageCount &&
          other.status == this.status &&
          other.updatedAt == this.updatedAt &&
          other.pending == this.pending);
}

class LocalProgressCompanion extends UpdateCompanion<LocalProgressData> {
  final Value<String> comicId;
  final Value<String> serverId;
  final Value<int> page;
  final Value<int> pageCount;
  final Value<String> status;
  final Value<DateTime> updatedAt;
  final Value<bool> pending;
  final Value<int> rowid;
  const LocalProgressCompanion({
    this.comicId = const Value.absent(),
    this.serverId = const Value.absent(),
    this.page = const Value.absent(),
    this.pageCount = const Value.absent(),
    this.status = const Value.absent(),
    this.updatedAt = const Value.absent(),
    this.pending = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  LocalProgressCompanion.insert({
    required String comicId,
    required String serverId,
    required int page,
    required int pageCount,
    required String status,
    required DateTime updatedAt,
    this.pending = const Value.absent(),
    this.rowid = const Value.absent(),
  }) : comicId = Value(comicId),
       serverId = Value(serverId),
       page = Value(page),
       pageCount = Value(pageCount),
       status = Value(status),
       updatedAt = Value(updatedAt);
  static Insertable<LocalProgressData> custom({
    Expression<String>? comicId,
    Expression<String>? serverId,
    Expression<int>? page,
    Expression<int>? pageCount,
    Expression<String>? status,
    Expression<DateTime>? updatedAt,
    Expression<bool>? pending,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (comicId != null) 'comic_id': comicId,
      if (serverId != null) 'server_id': serverId,
      if (page != null) 'page': page,
      if (pageCount != null) 'page_count': pageCount,
      if (status != null) 'status': status,
      if (updatedAt != null) 'updated_at': updatedAt,
      if (pending != null) 'pending': pending,
      if (rowid != null) 'rowid': rowid,
    });
  }

  LocalProgressCompanion copyWith({
    Value<String>? comicId,
    Value<String>? serverId,
    Value<int>? page,
    Value<int>? pageCount,
    Value<String>? status,
    Value<DateTime>? updatedAt,
    Value<bool>? pending,
    Value<int>? rowid,
  }) {
    return LocalProgressCompanion(
      comicId: comicId ?? this.comicId,
      serverId: serverId ?? this.serverId,
      page: page ?? this.page,
      pageCount: pageCount ?? this.pageCount,
      status: status ?? this.status,
      updatedAt: updatedAt ?? this.updatedAt,
      pending: pending ?? this.pending,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (comicId.present) {
      map['comic_id'] = Variable<String>(comicId.value);
    }
    if (serverId.present) {
      map['server_id'] = Variable<String>(serverId.value);
    }
    if (page.present) {
      map['page'] = Variable<int>(page.value);
    }
    if (pageCount.present) {
      map['page_count'] = Variable<int>(pageCount.value);
    }
    if (status.present) {
      map['status'] = Variable<String>(status.value);
    }
    if (updatedAt.present) {
      map['updated_at'] = Variable<DateTime>(updatedAt.value);
    }
    if (pending.present) {
      map['pending'] = Variable<bool>(pending.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('LocalProgressCompanion(')
          ..write('comicId: $comicId, ')
          ..write('serverId: $serverId, ')
          ..write('page: $page, ')
          ..write('pageCount: $pageCount, ')
          ..write('status: $status, ')
          ..write('updatedAt: $updatedAt, ')
          ..write('pending: $pending, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

class $FavoritesTable extends Favorites
    with TableInfo<$FavoritesTable, Favorite> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $FavoritesTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _serverIdMeta = const VerificationMeta(
    'serverId',
  );
  @override
  late final GeneratedColumn<String> serverId = GeneratedColumn<String>(
    'server_id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _comicIdMeta = const VerificationMeta(
    'comicId',
  );
  @override
  late final GeneratedColumn<String> comicId = GeneratedColumn<String>(
    'comic_id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  @override
  List<GeneratedColumn> get $columns => [serverId, comicId];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'favorites';
  @override
  VerificationContext validateIntegrity(
    Insertable<Favorite> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('server_id')) {
      context.handle(
        _serverIdMeta,
        serverId.isAcceptableOrUnknown(data['server_id']!, _serverIdMeta),
      );
    } else if (isInserting) {
      context.missing(_serverIdMeta);
    }
    if (data.containsKey('comic_id')) {
      context.handle(
        _comicIdMeta,
        comicId.isAcceptableOrUnknown(data['comic_id']!, _comicIdMeta),
      );
    } else if (isInserting) {
      context.missing(_comicIdMeta);
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {serverId, comicId};
  @override
  Favorite map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return Favorite(
      serverId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}server_id'],
      )!,
      comicId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}comic_id'],
      )!,
    );
  }

  @override
  $FavoritesTable createAlias(String alias) {
    return $FavoritesTable(attachedDatabase, alias);
  }
}

class Favorite extends DataClass implements Insertable<Favorite> {
  final String serverId;
  final String comicId;
  const Favorite({required this.serverId, required this.comicId});
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['server_id'] = Variable<String>(serverId);
    map['comic_id'] = Variable<String>(comicId);
    return map;
  }

  FavoritesCompanion toCompanion(bool nullToAbsent) {
    return FavoritesCompanion(
      serverId: Value(serverId),
      comicId: Value(comicId),
    );
  }

  factory Favorite.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return Favorite(
      serverId: serializer.fromJson<String>(json['serverId']),
      comicId: serializer.fromJson<String>(json['comicId']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'serverId': serializer.toJson<String>(serverId),
      'comicId': serializer.toJson<String>(comicId),
    };
  }

  Favorite copyWith({String? serverId, String? comicId}) => Favorite(
    serverId: serverId ?? this.serverId,
    comicId: comicId ?? this.comicId,
  );
  Favorite copyWithCompanion(FavoritesCompanion data) {
    return Favorite(
      serverId: data.serverId.present ? data.serverId.value : this.serverId,
      comicId: data.comicId.present ? data.comicId.value : this.comicId,
    );
  }

  @override
  String toString() {
    return (StringBuffer('Favorite(')
          ..write('serverId: $serverId, ')
          ..write('comicId: $comicId')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(serverId, comicId);
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is Favorite &&
          other.serverId == this.serverId &&
          other.comicId == this.comicId);
}

class FavoritesCompanion extends UpdateCompanion<Favorite> {
  final Value<String> serverId;
  final Value<String> comicId;
  final Value<int> rowid;
  const FavoritesCompanion({
    this.serverId = const Value.absent(),
    this.comicId = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  FavoritesCompanion.insert({
    required String serverId,
    required String comicId,
    this.rowid = const Value.absent(),
  }) : serverId = Value(serverId),
       comicId = Value(comicId);
  static Insertable<Favorite> custom({
    Expression<String>? serverId,
    Expression<String>? comicId,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (serverId != null) 'server_id': serverId,
      if (comicId != null) 'comic_id': comicId,
      if (rowid != null) 'rowid': rowid,
    });
  }

  FavoritesCompanion copyWith({
    Value<String>? serverId,
    Value<String>? comicId,
    Value<int>? rowid,
  }) {
    return FavoritesCompanion(
      serverId: serverId ?? this.serverId,
      comicId: comicId ?? this.comicId,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (serverId.present) {
      map['server_id'] = Variable<String>(serverId.value);
    }
    if (comicId.present) {
      map['comic_id'] = Variable<String>(comicId.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('FavoritesCompanion(')
          ..write('serverId: $serverId, ')
          ..write('comicId: $comicId, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

class $PreferencesTable extends Preferences
    with TableInfo<$PreferencesTable, Preference> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $PreferencesTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _keyMeta = const VerificationMeta('key');
  @override
  late final GeneratedColumn<String> key = GeneratedColumn<String>(
    'key',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _valueMeta = const VerificationMeta('value');
  @override
  late final GeneratedColumn<String> value = GeneratedColumn<String>(
    'value',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  @override
  List<GeneratedColumn> get $columns => [key, value];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'preferences';
  @override
  VerificationContext validateIntegrity(
    Insertable<Preference> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('key')) {
      context.handle(
        _keyMeta,
        key.isAcceptableOrUnknown(data['key']!, _keyMeta),
      );
    } else if (isInserting) {
      context.missing(_keyMeta);
    }
    if (data.containsKey('value')) {
      context.handle(
        _valueMeta,
        value.isAcceptableOrUnknown(data['value']!, _valueMeta),
      );
    } else if (isInserting) {
      context.missing(_valueMeta);
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {key};
  @override
  Preference map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return Preference(
      key: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}key'],
      )!,
      value: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}value'],
      )!,
    );
  }

  @override
  $PreferencesTable createAlias(String alias) {
    return $PreferencesTable(attachedDatabase, alias);
  }
}

class Preference extends DataClass implements Insertable<Preference> {
  final String key;
  final String value;
  const Preference({required this.key, required this.value});
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['key'] = Variable<String>(key);
    map['value'] = Variable<String>(value);
    return map;
  }

  PreferencesCompanion toCompanion(bool nullToAbsent) {
    return PreferencesCompanion(key: Value(key), value: Value(value));
  }

  factory Preference.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return Preference(
      key: serializer.fromJson<String>(json['key']),
      value: serializer.fromJson<String>(json['value']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'key': serializer.toJson<String>(key),
      'value': serializer.toJson<String>(value),
    };
  }

  Preference copyWith({String? key, String? value}) =>
      Preference(key: key ?? this.key, value: value ?? this.value);
  Preference copyWithCompanion(PreferencesCompanion data) {
    return Preference(
      key: data.key.present ? data.key.value : this.key,
      value: data.value.present ? data.value.value : this.value,
    );
  }

  @override
  String toString() {
    return (StringBuffer('Preference(')
          ..write('key: $key, ')
          ..write('value: $value')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(key, value);
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is Preference &&
          other.key == this.key &&
          other.value == this.value);
}

class PreferencesCompanion extends UpdateCompanion<Preference> {
  final Value<String> key;
  final Value<String> value;
  final Value<int> rowid;
  const PreferencesCompanion({
    this.key = const Value.absent(),
    this.value = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  PreferencesCompanion.insert({
    required String key,
    required String value,
    this.rowid = const Value.absent(),
  }) : key = Value(key),
       value = Value(value);
  static Insertable<Preference> custom({
    Expression<String>? key,
    Expression<String>? value,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (key != null) 'key': key,
      if (value != null) 'value': value,
      if (rowid != null) 'rowid': rowid,
    });
  }

  PreferencesCompanion copyWith({
    Value<String>? key,
    Value<String>? value,
    Value<int>? rowid,
  }) {
    return PreferencesCompanion(
      key: key ?? this.key,
      value: value ?? this.value,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (key.present) {
      map['key'] = Variable<String>(key.value);
    }
    if (value.present) {
      map['value'] = Variable<String>(value.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('PreferencesCompanion(')
          ..write('key: $key, ')
          ..write('value: $value, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

abstract class _$BoxDatabase extends GeneratedDatabase {
  _$BoxDatabase(QueryExecutor e) : super(e);
  $BoxDatabaseManager get managers => $BoxDatabaseManager(this);
  late final $CachedComicsTable cachedComics = $CachedComicsTable(this);
  late final $CachedSeriesTable cachedSeries = $CachedSeriesTable(this);
  late final $CachedFoldersTable cachedFolders = $CachedFoldersTable(this);
  late final $CachedLibrariesTable cachedLibraries = $CachedLibrariesTable(
    this,
  );
  late final $LocalProgressTable localProgress = $LocalProgressTable(this);
  late final $FavoritesTable favorites = $FavoritesTable(this);
  late final $PreferencesTable preferences = $PreferencesTable(this);
  @override
  Iterable<TableInfo<Table, Object?>> get allTables =>
      allSchemaEntities.whereType<TableInfo<Table, Object?>>();
  @override
  List<DatabaseSchemaEntity> get allSchemaEntities => [
    cachedComics,
    cachedSeries,
    cachedFolders,
    cachedLibraries,
    localProgress,
    favorites,
    preferences,
  ];
}

typedef $$CachedComicsTableCreateCompanionBuilder =
    CachedComicsCompanion Function({
      required String id,
      required String serverId,
      required String libraryId,
      required String title,
      Value<String?> seriesId,
      Value<String> seriesName,
      Value<String> number,
      Value<String> folderPath,
      Value<int> pageCount,
      Value<String> coverPath,
      Value<String?> coverPlaceholder,
      Value<int> fileSize,
      Value<DateTime?> createdAt,
      required DateTime cachedAt,
      Value<int> rowid,
    });
typedef $$CachedComicsTableUpdateCompanionBuilder =
    CachedComicsCompanion Function({
      Value<String> id,
      Value<String> serverId,
      Value<String> libraryId,
      Value<String> title,
      Value<String?> seriesId,
      Value<String> seriesName,
      Value<String> number,
      Value<String> folderPath,
      Value<int> pageCount,
      Value<String> coverPath,
      Value<String?> coverPlaceholder,
      Value<int> fileSize,
      Value<DateTime?> createdAt,
      Value<DateTime> cachedAt,
      Value<int> rowid,
    });

class $$CachedComicsTableFilterComposer
    extends Composer<_$BoxDatabase, $CachedComicsTable> {
  $$CachedComicsTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get serverId => $composableBuilder(
    column: $table.serverId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get libraryId => $composableBuilder(
    column: $table.libraryId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get title => $composableBuilder(
    column: $table.title,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get seriesId => $composableBuilder(
    column: $table.seriesId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get seriesName => $composableBuilder(
    column: $table.seriesName,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get number => $composableBuilder(
    column: $table.number,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get folderPath => $composableBuilder(
    column: $table.folderPath,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<int> get pageCount => $composableBuilder(
    column: $table.pageCount,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get coverPath => $composableBuilder(
    column: $table.coverPath,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get coverPlaceholder => $composableBuilder(
    column: $table.coverPlaceholder,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<int> get fileSize => $composableBuilder(
    column: $table.fileSize,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<DateTime> get createdAt => $composableBuilder(
    column: $table.createdAt,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<DateTime> get cachedAt => $composableBuilder(
    column: $table.cachedAt,
    builder: (column) => ColumnFilters(column),
  );
}

class $$CachedComicsTableOrderingComposer
    extends Composer<_$BoxDatabase, $CachedComicsTable> {
  $$CachedComicsTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get serverId => $composableBuilder(
    column: $table.serverId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get libraryId => $composableBuilder(
    column: $table.libraryId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get title => $composableBuilder(
    column: $table.title,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get seriesId => $composableBuilder(
    column: $table.seriesId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get seriesName => $composableBuilder(
    column: $table.seriesName,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get number => $composableBuilder(
    column: $table.number,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get folderPath => $composableBuilder(
    column: $table.folderPath,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<int> get pageCount => $composableBuilder(
    column: $table.pageCount,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get coverPath => $composableBuilder(
    column: $table.coverPath,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get coverPlaceholder => $composableBuilder(
    column: $table.coverPlaceholder,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<int> get fileSize => $composableBuilder(
    column: $table.fileSize,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<DateTime> get createdAt => $composableBuilder(
    column: $table.createdAt,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<DateTime> get cachedAt => $composableBuilder(
    column: $table.cachedAt,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$CachedComicsTableAnnotationComposer
    extends Composer<_$BoxDatabase, $CachedComicsTable> {
  $$CachedComicsTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get id =>
      $composableBuilder(column: $table.id, builder: (column) => column);

  GeneratedColumn<String> get serverId =>
      $composableBuilder(column: $table.serverId, builder: (column) => column);

  GeneratedColumn<String> get libraryId =>
      $composableBuilder(column: $table.libraryId, builder: (column) => column);

  GeneratedColumn<String> get title =>
      $composableBuilder(column: $table.title, builder: (column) => column);

  GeneratedColumn<String> get seriesId =>
      $composableBuilder(column: $table.seriesId, builder: (column) => column);

  GeneratedColumn<String> get seriesName => $composableBuilder(
    column: $table.seriesName,
    builder: (column) => column,
  );

  GeneratedColumn<String> get number =>
      $composableBuilder(column: $table.number, builder: (column) => column);

  GeneratedColumn<String> get folderPath => $composableBuilder(
    column: $table.folderPath,
    builder: (column) => column,
  );

  GeneratedColumn<int> get pageCount =>
      $composableBuilder(column: $table.pageCount, builder: (column) => column);

  GeneratedColumn<String> get coverPath =>
      $composableBuilder(column: $table.coverPath, builder: (column) => column);

  GeneratedColumn<String> get coverPlaceholder => $composableBuilder(
    column: $table.coverPlaceholder,
    builder: (column) => column,
  );

  GeneratedColumn<int> get fileSize =>
      $composableBuilder(column: $table.fileSize, builder: (column) => column);

  GeneratedColumn<DateTime> get createdAt =>
      $composableBuilder(column: $table.createdAt, builder: (column) => column);

  GeneratedColumn<DateTime> get cachedAt =>
      $composableBuilder(column: $table.cachedAt, builder: (column) => column);
}

class $$CachedComicsTableTableManager
    extends
        RootTableManager<
          _$BoxDatabase,
          $CachedComicsTable,
          CachedComic,
          $$CachedComicsTableFilterComposer,
          $$CachedComicsTableOrderingComposer,
          $$CachedComicsTableAnnotationComposer,
          $$CachedComicsTableCreateCompanionBuilder,
          $$CachedComicsTableUpdateCompanionBuilder,
          (
            CachedComic,
            BaseReferences<_$BoxDatabase, $CachedComicsTable, CachedComic>,
          ),
          CachedComic,
          PrefetchHooks Function()
        > {
  $$CachedComicsTableTableManager(_$BoxDatabase db, $CachedComicsTable table)
    : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$CachedComicsTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$CachedComicsTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$CachedComicsTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<String> id = const Value.absent(),
                Value<String> serverId = const Value.absent(),
                Value<String> libraryId = const Value.absent(),
                Value<String> title = const Value.absent(),
                Value<String?> seriesId = const Value.absent(),
                Value<String> seriesName = const Value.absent(),
                Value<String> number = const Value.absent(),
                Value<String> folderPath = const Value.absent(),
                Value<int> pageCount = const Value.absent(),
                Value<String> coverPath = const Value.absent(),
                Value<String?> coverPlaceholder = const Value.absent(),
                Value<int> fileSize = const Value.absent(),
                Value<DateTime?> createdAt = const Value.absent(),
                Value<DateTime> cachedAt = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => CachedComicsCompanion(
                id: id,
                serverId: serverId,
                libraryId: libraryId,
                title: title,
                seriesId: seriesId,
                seriesName: seriesName,
                number: number,
                folderPath: folderPath,
                pageCount: pageCount,
                coverPath: coverPath,
                coverPlaceholder: coverPlaceholder,
                fileSize: fileSize,
                createdAt: createdAt,
                cachedAt: cachedAt,
                rowid: rowid,
              ),
          createCompanionCallback:
              ({
                required String id,
                required String serverId,
                required String libraryId,
                required String title,
                Value<String?> seriesId = const Value.absent(),
                Value<String> seriesName = const Value.absent(),
                Value<String> number = const Value.absent(),
                Value<String> folderPath = const Value.absent(),
                Value<int> pageCount = const Value.absent(),
                Value<String> coverPath = const Value.absent(),
                Value<String?> coverPlaceholder = const Value.absent(),
                Value<int> fileSize = const Value.absent(),
                Value<DateTime?> createdAt = const Value.absent(),
                required DateTime cachedAt,
                Value<int> rowid = const Value.absent(),
              }) => CachedComicsCompanion.insert(
                id: id,
                serverId: serverId,
                libraryId: libraryId,
                title: title,
                seriesId: seriesId,
                seriesName: seriesName,
                number: number,
                folderPath: folderPath,
                pageCount: pageCount,
                coverPath: coverPath,
                coverPlaceholder: coverPlaceholder,
                fileSize: fileSize,
                createdAt: createdAt,
                cachedAt: cachedAt,
                rowid: rowid,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$CachedComicsTableProcessedTableManager =
    ProcessedTableManager<
      _$BoxDatabase,
      $CachedComicsTable,
      CachedComic,
      $$CachedComicsTableFilterComposer,
      $$CachedComicsTableOrderingComposer,
      $$CachedComicsTableAnnotationComposer,
      $$CachedComicsTableCreateCompanionBuilder,
      $$CachedComicsTableUpdateCompanionBuilder,
      (
        CachedComic,
        BaseReferences<_$BoxDatabase, $CachedComicsTable, CachedComic>,
      ),
      CachedComic,
      PrefetchHooks Function()
    >;
typedef $$CachedSeriesTableCreateCompanionBuilder =
    CachedSeriesCompanion Function({
      required String id,
      required String serverId,
      required String libraryId,
      required String name,
      Value<int> comicCount,
      Value<String> coverPath,
      Value<int> rowid,
    });
typedef $$CachedSeriesTableUpdateCompanionBuilder =
    CachedSeriesCompanion Function({
      Value<String> id,
      Value<String> serverId,
      Value<String> libraryId,
      Value<String> name,
      Value<int> comicCount,
      Value<String> coverPath,
      Value<int> rowid,
    });

class $$CachedSeriesTableFilterComposer
    extends Composer<_$BoxDatabase, $CachedSeriesTable> {
  $$CachedSeriesTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get serverId => $composableBuilder(
    column: $table.serverId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get libraryId => $composableBuilder(
    column: $table.libraryId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get name => $composableBuilder(
    column: $table.name,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<int> get comicCount => $composableBuilder(
    column: $table.comicCount,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get coverPath => $composableBuilder(
    column: $table.coverPath,
    builder: (column) => ColumnFilters(column),
  );
}

class $$CachedSeriesTableOrderingComposer
    extends Composer<_$BoxDatabase, $CachedSeriesTable> {
  $$CachedSeriesTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get serverId => $composableBuilder(
    column: $table.serverId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get libraryId => $composableBuilder(
    column: $table.libraryId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get name => $composableBuilder(
    column: $table.name,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<int> get comicCount => $composableBuilder(
    column: $table.comicCount,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get coverPath => $composableBuilder(
    column: $table.coverPath,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$CachedSeriesTableAnnotationComposer
    extends Composer<_$BoxDatabase, $CachedSeriesTable> {
  $$CachedSeriesTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get id =>
      $composableBuilder(column: $table.id, builder: (column) => column);

  GeneratedColumn<String> get serverId =>
      $composableBuilder(column: $table.serverId, builder: (column) => column);

  GeneratedColumn<String> get libraryId =>
      $composableBuilder(column: $table.libraryId, builder: (column) => column);

  GeneratedColumn<String> get name =>
      $composableBuilder(column: $table.name, builder: (column) => column);

  GeneratedColumn<int> get comicCount => $composableBuilder(
    column: $table.comicCount,
    builder: (column) => column,
  );

  GeneratedColumn<String> get coverPath =>
      $composableBuilder(column: $table.coverPath, builder: (column) => column);
}

class $$CachedSeriesTableTableManager
    extends
        RootTableManager<
          _$BoxDatabase,
          $CachedSeriesTable,
          CachedSeriesRow,
          $$CachedSeriesTableFilterComposer,
          $$CachedSeriesTableOrderingComposer,
          $$CachedSeriesTableAnnotationComposer,
          $$CachedSeriesTableCreateCompanionBuilder,
          $$CachedSeriesTableUpdateCompanionBuilder,
          (
            CachedSeriesRow,
            BaseReferences<_$BoxDatabase, $CachedSeriesTable, CachedSeriesRow>,
          ),
          CachedSeriesRow,
          PrefetchHooks Function()
        > {
  $$CachedSeriesTableTableManager(_$BoxDatabase db, $CachedSeriesTable table)
    : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$CachedSeriesTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$CachedSeriesTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$CachedSeriesTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<String> id = const Value.absent(),
                Value<String> serverId = const Value.absent(),
                Value<String> libraryId = const Value.absent(),
                Value<String> name = const Value.absent(),
                Value<int> comicCount = const Value.absent(),
                Value<String> coverPath = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => CachedSeriesCompanion(
                id: id,
                serverId: serverId,
                libraryId: libraryId,
                name: name,
                comicCount: comicCount,
                coverPath: coverPath,
                rowid: rowid,
              ),
          createCompanionCallback:
              ({
                required String id,
                required String serverId,
                required String libraryId,
                required String name,
                Value<int> comicCount = const Value.absent(),
                Value<String> coverPath = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => CachedSeriesCompanion.insert(
                id: id,
                serverId: serverId,
                libraryId: libraryId,
                name: name,
                comicCount: comicCount,
                coverPath: coverPath,
                rowid: rowid,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$CachedSeriesTableProcessedTableManager =
    ProcessedTableManager<
      _$BoxDatabase,
      $CachedSeriesTable,
      CachedSeriesRow,
      $$CachedSeriesTableFilterComposer,
      $$CachedSeriesTableOrderingComposer,
      $$CachedSeriesTableAnnotationComposer,
      $$CachedSeriesTableCreateCompanionBuilder,
      $$CachedSeriesTableUpdateCompanionBuilder,
      (
        CachedSeriesRow,
        BaseReferences<_$BoxDatabase, $CachedSeriesTable, CachedSeriesRow>,
      ),
      CachedSeriesRow,
      PrefetchHooks Function()
    >;
typedef $$CachedFoldersTableCreateCompanionBuilder =
    CachedFoldersCompanion Function({
      required String serverId,
      required String libraryId,
      required String path,
      required String name,
      required int depth,
      Value<int> comicCount,
      Value<int> rowid,
    });
typedef $$CachedFoldersTableUpdateCompanionBuilder =
    CachedFoldersCompanion Function({
      Value<String> serverId,
      Value<String> libraryId,
      Value<String> path,
      Value<String> name,
      Value<int> depth,
      Value<int> comicCount,
      Value<int> rowid,
    });

class $$CachedFoldersTableFilterComposer
    extends Composer<_$BoxDatabase, $CachedFoldersTable> {
  $$CachedFoldersTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get serverId => $composableBuilder(
    column: $table.serverId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get libraryId => $composableBuilder(
    column: $table.libraryId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get path => $composableBuilder(
    column: $table.path,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get name => $composableBuilder(
    column: $table.name,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<int> get depth => $composableBuilder(
    column: $table.depth,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<int> get comicCount => $composableBuilder(
    column: $table.comicCount,
    builder: (column) => ColumnFilters(column),
  );
}

class $$CachedFoldersTableOrderingComposer
    extends Composer<_$BoxDatabase, $CachedFoldersTable> {
  $$CachedFoldersTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get serverId => $composableBuilder(
    column: $table.serverId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get libraryId => $composableBuilder(
    column: $table.libraryId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get path => $composableBuilder(
    column: $table.path,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get name => $composableBuilder(
    column: $table.name,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<int> get depth => $composableBuilder(
    column: $table.depth,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<int> get comicCount => $composableBuilder(
    column: $table.comicCount,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$CachedFoldersTableAnnotationComposer
    extends Composer<_$BoxDatabase, $CachedFoldersTable> {
  $$CachedFoldersTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get serverId =>
      $composableBuilder(column: $table.serverId, builder: (column) => column);

  GeneratedColumn<String> get libraryId =>
      $composableBuilder(column: $table.libraryId, builder: (column) => column);

  GeneratedColumn<String> get path =>
      $composableBuilder(column: $table.path, builder: (column) => column);

  GeneratedColumn<String> get name =>
      $composableBuilder(column: $table.name, builder: (column) => column);

  GeneratedColumn<int> get depth =>
      $composableBuilder(column: $table.depth, builder: (column) => column);

  GeneratedColumn<int> get comicCount => $composableBuilder(
    column: $table.comicCount,
    builder: (column) => column,
  );
}

class $$CachedFoldersTableTableManager
    extends
        RootTableManager<
          _$BoxDatabase,
          $CachedFoldersTable,
          CachedFolder,
          $$CachedFoldersTableFilterComposer,
          $$CachedFoldersTableOrderingComposer,
          $$CachedFoldersTableAnnotationComposer,
          $$CachedFoldersTableCreateCompanionBuilder,
          $$CachedFoldersTableUpdateCompanionBuilder,
          (
            CachedFolder,
            BaseReferences<_$BoxDatabase, $CachedFoldersTable, CachedFolder>,
          ),
          CachedFolder,
          PrefetchHooks Function()
        > {
  $$CachedFoldersTableTableManager(_$BoxDatabase db, $CachedFoldersTable table)
    : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$CachedFoldersTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$CachedFoldersTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$CachedFoldersTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<String> serverId = const Value.absent(),
                Value<String> libraryId = const Value.absent(),
                Value<String> path = const Value.absent(),
                Value<String> name = const Value.absent(),
                Value<int> depth = const Value.absent(),
                Value<int> comicCount = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => CachedFoldersCompanion(
                serverId: serverId,
                libraryId: libraryId,
                path: path,
                name: name,
                depth: depth,
                comicCount: comicCount,
                rowid: rowid,
              ),
          createCompanionCallback:
              ({
                required String serverId,
                required String libraryId,
                required String path,
                required String name,
                required int depth,
                Value<int> comicCount = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => CachedFoldersCompanion.insert(
                serverId: serverId,
                libraryId: libraryId,
                path: path,
                name: name,
                depth: depth,
                comicCount: comicCount,
                rowid: rowid,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$CachedFoldersTableProcessedTableManager =
    ProcessedTableManager<
      _$BoxDatabase,
      $CachedFoldersTable,
      CachedFolder,
      $$CachedFoldersTableFilterComposer,
      $$CachedFoldersTableOrderingComposer,
      $$CachedFoldersTableAnnotationComposer,
      $$CachedFoldersTableCreateCompanionBuilder,
      $$CachedFoldersTableUpdateCompanionBuilder,
      (
        CachedFolder,
        BaseReferences<_$BoxDatabase, $CachedFoldersTable, CachedFolder>,
      ),
      CachedFolder,
      PrefetchHooks Function()
    >;
typedef $$CachedLibrariesTableCreateCompanionBuilder =
    CachedLibrariesCompanion Function({
      required String id,
      required String serverId,
      required String name,
      Value<int> comicCount,
      Value<int> rowid,
    });
typedef $$CachedLibrariesTableUpdateCompanionBuilder =
    CachedLibrariesCompanion Function({
      Value<String> id,
      Value<String> serverId,
      Value<String> name,
      Value<int> comicCount,
      Value<int> rowid,
    });

class $$CachedLibrariesTableFilterComposer
    extends Composer<_$BoxDatabase, $CachedLibrariesTable> {
  $$CachedLibrariesTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get serverId => $composableBuilder(
    column: $table.serverId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get name => $composableBuilder(
    column: $table.name,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<int> get comicCount => $composableBuilder(
    column: $table.comicCount,
    builder: (column) => ColumnFilters(column),
  );
}

class $$CachedLibrariesTableOrderingComposer
    extends Composer<_$BoxDatabase, $CachedLibrariesTable> {
  $$CachedLibrariesTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get serverId => $composableBuilder(
    column: $table.serverId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get name => $composableBuilder(
    column: $table.name,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<int> get comicCount => $composableBuilder(
    column: $table.comicCount,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$CachedLibrariesTableAnnotationComposer
    extends Composer<_$BoxDatabase, $CachedLibrariesTable> {
  $$CachedLibrariesTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get id =>
      $composableBuilder(column: $table.id, builder: (column) => column);

  GeneratedColumn<String> get serverId =>
      $composableBuilder(column: $table.serverId, builder: (column) => column);

  GeneratedColumn<String> get name =>
      $composableBuilder(column: $table.name, builder: (column) => column);

  GeneratedColumn<int> get comicCount => $composableBuilder(
    column: $table.comicCount,
    builder: (column) => column,
  );
}

class $$CachedLibrariesTableTableManager
    extends
        RootTableManager<
          _$BoxDatabase,
          $CachedLibrariesTable,
          CachedLibrary,
          $$CachedLibrariesTableFilterComposer,
          $$CachedLibrariesTableOrderingComposer,
          $$CachedLibrariesTableAnnotationComposer,
          $$CachedLibrariesTableCreateCompanionBuilder,
          $$CachedLibrariesTableUpdateCompanionBuilder,
          (
            CachedLibrary,
            BaseReferences<_$BoxDatabase, $CachedLibrariesTable, CachedLibrary>,
          ),
          CachedLibrary,
          PrefetchHooks Function()
        > {
  $$CachedLibrariesTableTableManager(
    _$BoxDatabase db,
    $CachedLibrariesTable table,
  ) : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$CachedLibrariesTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$CachedLibrariesTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$CachedLibrariesTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<String> id = const Value.absent(),
                Value<String> serverId = const Value.absent(),
                Value<String> name = const Value.absent(),
                Value<int> comicCount = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => CachedLibrariesCompanion(
                id: id,
                serverId: serverId,
                name: name,
                comicCount: comicCount,
                rowid: rowid,
              ),
          createCompanionCallback:
              ({
                required String id,
                required String serverId,
                required String name,
                Value<int> comicCount = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => CachedLibrariesCompanion.insert(
                id: id,
                serverId: serverId,
                name: name,
                comicCount: comicCount,
                rowid: rowid,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$CachedLibrariesTableProcessedTableManager =
    ProcessedTableManager<
      _$BoxDatabase,
      $CachedLibrariesTable,
      CachedLibrary,
      $$CachedLibrariesTableFilterComposer,
      $$CachedLibrariesTableOrderingComposer,
      $$CachedLibrariesTableAnnotationComposer,
      $$CachedLibrariesTableCreateCompanionBuilder,
      $$CachedLibrariesTableUpdateCompanionBuilder,
      (
        CachedLibrary,
        BaseReferences<_$BoxDatabase, $CachedLibrariesTable, CachedLibrary>,
      ),
      CachedLibrary,
      PrefetchHooks Function()
    >;
typedef $$LocalProgressTableCreateCompanionBuilder =
    LocalProgressCompanion Function({
      required String comicId,
      required String serverId,
      required int page,
      required int pageCount,
      required String status,
      required DateTime updatedAt,
      Value<bool> pending,
      Value<int> rowid,
    });
typedef $$LocalProgressTableUpdateCompanionBuilder =
    LocalProgressCompanion Function({
      Value<String> comicId,
      Value<String> serverId,
      Value<int> page,
      Value<int> pageCount,
      Value<String> status,
      Value<DateTime> updatedAt,
      Value<bool> pending,
      Value<int> rowid,
    });

class $$LocalProgressTableFilterComposer
    extends Composer<_$BoxDatabase, $LocalProgressTable> {
  $$LocalProgressTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get comicId => $composableBuilder(
    column: $table.comicId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get serverId => $composableBuilder(
    column: $table.serverId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<int> get page => $composableBuilder(
    column: $table.page,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<int> get pageCount => $composableBuilder(
    column: $table.pageCount,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get status => $composableBuilder(
    column: $table.status,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<DateTime> get updatedAt => $composableBuilder(
    column: $table.updatedAt,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<bool> get pending => $composableBuilder(
    column: $table.pending,
    builder: (column) => ColumnFilters(column),
  );
}

class $$LocalProgressTableOrderingComposer
    extends Composer<_$BoxDatabase, $LocalProgressTable> {
  $$LocalProgressTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get comicId => $composableBuilder(
    column: $table.comicId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get serverId => $composableBuilder(
    column: $table.serverId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<int> get page => $composableBuilder(
    column: $table.page,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<int> get pageCount => $composableBuilder(
    column: $table.pageCount,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get status => $composableBuilder(
    column: $table.status,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<DateTime> get updatedAt => $composableBuilder(
    column: $table.updatedAt,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<bool> get pending => $composableBuilder(
    column: $table.pending,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$LocalProgressTableAnnotationComposer
    extends Composer<_$BoxDatabase, $LocalProgressTable> {
  $$LocalProgressTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get comicId =>
      $composableBuilder(column: $table.comicId, builder: (column) => column);

  GeneratedColumn<String> get serverId =>
      $composableBuilder(column: $table.serverId, builder: (column) => column);

  GeneratedColumn<int> get page =>
      $composableBuilder(column: $table.page, builder: (column) => column);

  GeneratedColumn<int> get pageCount =>
      $composableBuilder(column: $table.pageCount, builder: (column) => column);

  GeneratedColumn<String> get status =>
      $composableBuilder(column: $table.status, builder: (column) => column);

  GeneratedColumn<DateTime> get updatedAt =>
      $composableBuilder(column: $table.updatedAt, builder: (column) => column);

  GeneratedColumn<bool> get pending =>
      $composableBuilder(column: $table.pending, builder: (column) => column);
}

class $$LocalProgressTableTableManager
    extends
        RootTableManager<
          _$BoxDatabase,
          $LocalProgressTable,
          LocalProgressData,
          $$LocalProgressTableFilterComposer,
          $$LocalProgressTableOrderingComposer,
          $$LocalProgressTableAnnotationComposer,
          $$LocalProgressTableCreateCompanionBuilder,
          $$LocalProgressTableUpdateCompanionBuilder,
          (
            LocalProgressData,
            BaseReferences<
              _$BoxDatabase,
              $LocalProgressTable,
              LocalProgressData
            >,
          ),
          LocalProgressData,
          PrefetchHooks Function()
        > {
  $$LocalProgressTableTableManager(_$BoxDatabase db, $LocalProgressTable table)
    : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$LocalProgressTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$LocalProgressTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$LocalProgressTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<String> comicId = const Value.absent(),
                Value<String> serverId = const Value.absent(),
                Value<int> page = const Value.absent(),
                Value<int> pageCount = const Value.absent(),
                Value<String> status = const Value.absent(),
                Value<DateTime> updatedAt = const Value.absent(),
                Value<bool> pending = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => LocalProgressCompanion(
                comicId: comicId,
                serverId: serverId,
                page: page,
                pageCount: pageCount,
                status: status,
                updatedAt: updatedAt,
                pending: pending,
                rowid: rowid,
              ),
          createCompanionCallback:
              ({
                required String comicId,
                required String serverId,
                required int page,
                required int pageCount,
                required String status,
                required DateTime updatedAt,
                Value<bool> pending = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => LocalProgressCompanion.insert(
                comicId: comicId,
                serverId: serverId,
                page: page,
                pageCount: pageCount,
                status: status,
                updatedAt: updatedAt,
                pending: pending,
                rowid: rowid,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$LocalProgressTableProcessedTableManager =
    ProcessedTableManager<
      _$BoxDatabase,
      $LocalProgressTable,
      LocalProgressData,
      $$LocalProgressTableFilterComposer,
      $$LocalProgressTableOrderingComposer,
      $$LocalProgressTableAnnotationComposer,
      $$LocalProgressTableCreateCompanionBuilder,
      $$LocalProgressTableUpdateCompanionBuilder,
      (
        LocalProgressData,
        BaseReferences<_$BoxDatabase, $LocalProgressTable, LocalProgressData>,
      ),
      LocalProgressData,
      PrefetchHooks Function()
    >;
typedef $$FavoritesTableCreateCompanionBuilder =
    FavoritesCompanion Function({
      required String serverId,
      required String comicId,
      Value<int> rowid,
    });
typedef $$FavoritesTableUpdateCompanionBuilder =
    FavoritesCompanion Function({
      Value<String> serverId,
      Value<String> comicId,
      Value<int> rowid,
    });

class $$FavoritesTableFilterComposer
    extends Composer<_$BoxDatabase, $FavoritesTable> {
  $$FavoritesTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get serverId => $composableBuilder(
    column: $table.serverId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get comicId => $composableBuilder(
    column: $table.comicId,
    builder: (column) => ColumnFilters(column),
  );
}

class $$FavoritesTableOrderingComposer
    extends Composer<_$BoxDatabase, $FavoritesTable> {
  $$FavoritesTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get serverId => $composableBuilder(
    column: $table.serverId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get comicId => $composableBuilder(
    column: $table.comicId,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$FavoritesTableAnnotationComposer
    extends Composer<_$BoxDatabase, $FavoritesTable> {
  $$FavoritesTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get serverId =>
      $composableBuilder(column: $table.serverId, builder: (column) => column);

  GeneratedColumn<String> get comicId =>
      $composableBuilder(column: $table.comicId, builder: (column) => column);
}

class $$FavoritesTableTableManager
    extends
        RootTableManager<
          _$BoxDatabase,
          $FavoritesTable,
          Favorite,
          $$FavoritesTableFilterComposer,
          $$FavoritesTableOrderingComposer,
          $$FavoritesTableAnnotationComposer,
          $$FavoritesTableCreateCompanionBuilder,
          $$FavoritesTableUpdateCompanionBuilder,
          (Favorite, BaseReferences<_$BoxDatabase, $FavoritesTable, Favorite>),
          Favorite,
          PrefetchHooks Function()
        > {
  $$FavoritesTableTableManager(_$BoxDatabase db, $FavoritesTable table)
    : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$FavoritesTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$FavoritesTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$FavoritesTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<String> serverId = const Value.absent(),
                Value<String> comicId = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => FavoritesCompanion(
                serverId: serverId,
                comicId: comicId,
                rowid: rowid,
              ),
          createCompanionCallback:
              ({
                required String serverId,
                required String comicId,
                Value<int> rowid = const Value.absent(),
              }) => FavoritesCompanion.insert(
                serverId: serverId,
                comicId: comicId,
                rowid: rowid,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$FavoritesTableProcessedTableManager =
    ProcessedTableManager<
      _$BoxDatabase,
      $FavoritesTable,
      Favorite,
      $$FavoritesTableFilterComposer,
      $$FavoritesTableOrderingComposer,
      $$FavoritesTableAnnotationComposer,
      $$FavoritesTableCreateCompanionBuilder,
      $$FavoritesTableUpdateCompanionBuilder,
      (Favorite, BaseReferences<_$BoxDatabase, $FavoritesTable, Favorite>),
      Favorite,
      PrefetchHooks Function()
    >;
typedef $$PreferencesTableCreateCompanionBuilder =
    PreferencesCompanion Function({
      required String key,
      required String value,
      Value<int> rowid,
    });
typedef $$PreferencesTableUpdateCompanionBuilder =
    PreferencesCompanion Function({
      Value<String> key,
      Value<String> value,
      Value<int> rowid,
    });

class $$PreferencesTableFilterComposer
    extends Composer<_$BoxDatabase, $PreferencesTable> {
  $$PreferencesTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get key => $composableBuilder(
    column: $table.key,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get value => $composableBuilder(
    column: $table.value,
    builder: (column) => ColumnFilters(column),
  );
}

class $$PreferencesTableOrderingComposer
    extends Composer<_$BoxDatabase, $PreferencesTable> {
  $$PreferencesTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get key => $composableBuilder(
    column: $table.key,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get value => $composableBuilder(
    column: $table.value,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$PreferencesTableAnnotationComposer
    extends Composer<_$BoxDatabase, $PreferencesTable> {
  $$PreferencesTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get key =>
      $composableBuilder(column: $table.key, builder: (column) => column);

  GeneratedColumn<String> get value =>
      $composableBuilder(column: $table.value, builder: (column) => column);
}

class $$PreferencesTableTableManager
    extends
        RootTableManager<
          _$BoxDatabase,
          $PreferencesTable,
          Preference,
          $$PreferencesTableFilterComposer,
          $$PreferencesTableOrderingComposer,
          $$PreferencesTableAnnotationComposer,
          $$PreferencesTableCreateCompanionBuilder,
          $$PreferencesTableUpdateCompanionBuilder,
          (
            Preference,
            BaseReferences<_$BoxDatabase, $PreferencesTable, Preference>,
          ),
          Preference,
          PrefetchHooks Function()
        > {
  $$PreferencesTableTableManager(_$BoxDatabase db, $PreferencesTable table)
    : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$PreferencesTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$PreferencesTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$PreferencesTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<String> key = const Value.absent(),
                Value<String> value = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => PreferencesCompanion(key: key, value: value, rowid: rowid),
          createCompanionCallback:
              ({
                required String key,
                required String value,
                Value<int> rowid = const Value.absent(),
              }) => PreferencesCompanion.insert(
                key: key,
                value: value,
                rowid: rowid,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$PreferencesTableProcessedTableManager =
    ProcessedTableManager<
      _$BoxDatabase,
      $PreferencesTable,
      Preference,
      $$PreferencesTableFilterComposer,
      $$PreferencesTableOrderingComposer,
      $$PreferencesTableAnnotationComposer,
      $$PreferencesTableCreateCompanionBuilder,
      $$PreferencesTableUpdateCompanionBuilder,
      (
        Preference,
        BaseReferences<_$BoxDatabase, $PreferencesTable, Preference>,
      ),
      Preference,
      PrefetchHooks Function()
    >;

class $BoxDatabaseManager {
  final _$BoxDatabase _db;
  $BoxDatabaseManager(this._db);
  $$CachedComicsTableTableManager get cachedComics =>
      $$CachedComicsTableTableManager(_db, _db.cachedComics);
  $$CachedSeriesTableTableManager get cachedSeries =>
      $$CachedSeriesTableTableManager(_db, _db.cachedSeries);
  $$CachedFoldersTableTableManager get cachedFolders =>
      $$CachedFoldersTableTableManager(_db, _db.cachedFolders);
  $$CachedLibrariesTableTableManager get cachedLibraries =>
      $$CachedLibrariesTableTableManager(_db, _db.cachedLibraries);
  $$LocalProgressTableTableManager get localProgress =>
      $$LocalProgressTableTableManager(_db, _db.localProgress);
  $$FavoritesTableTableManager get favorites =>
      $$FavoritesTableTableManager(_db, _db.favorites);
  $$PreferencesTableTableManager get preferences =>
      $$PreferencesTableTableManager(_db, _db.preferences);
}
